//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const processCommandLineInformation = 60

var (
	ntdll                         = windows.NewLazySystemDLL("ntdll.dll")
	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
	webViewProfilePollInterval    = 150 * time.Millisecond
	findWebViewProfileProcesses   = webViewProcessesUsingProfile
	reportedWebViewQueryFailures  sync.Map
)

type nativeUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

// waitForWebViewProfileRelease prevents a freshly updated process from opening
// the same WebView2 user-data folder while renderer/storage children from the
// previous process are still shutting down. A fixed sleep is not sufficient on
// slower machines and can leave the replacement WebView permanently blank.
func waitForWebViewProfileRelease(dataPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	clearChecks := 0
	for {
		processes, err := findWebViewProfileProcesses(dataPath)
		if err != nil {
			return err
		}
		if len(processes) == 0 {
			// Require two clear snapshots so a child that is being respawned during
			// WebView2 shutdown cannot slip through a single empty sample.
			clearChecks++
			if clearChecks >= 2 {
				return nil
			}
		} else {
			clearChecks = 0
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("WebView2 profile is still used by processes %v after %s", processes, timeout)
		}
		time.Sleep(webViewProfilePollInterval)
	}
}

func webViewProcessesUsingProfile(dataPath string) ([]uint32, error) {
	target := normalizeCommandLinePath(dataPath)
	if target == "" {
		return nil, errors.New("empty WebView2 data path")
	}
	// WebView2 appends EBWebView to the user-data path supplied by the host.
	targetWithSuffix := normalizeCommandLinePath(filepath.Join(dataPath, "EBWebView"))
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}
	currentPID := uint32(os.Getpid())
	var matches []uint32
	for {
		if entry.ProcessID != currentPID && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "msedgewebview2.exe") {
			commandLine, commandErr := queryProcessCommandLine(entry.ProcessID)
			if commandErr != nil {
				// Protected WebView2 instances owned by Windows may not expose their
				// command line to a normal desktop process. They must not block every
				// update, but record the uncertainty; Dilmeter's same-user WebView2
				// children are queryable and are matched by their exact profile arg.
				if _, alreadyReported := reportedWebViewQueryFailures.LoadOrStore(entry.ProcessID, struct{}{}); !alreadyReported {
					logger.Printf("inspect WebView2 process %d failed: %v", entry.ProcessID, commandErr)
				}
			} else if commandLineUsesWebViewProfile(commandLine, target, targetWithSuffix) {
				matches = append(matches, entry.ProcessID)
			}
		}
		entry.Size = uint32(unsafe.Sizeof(windows.ProcessEntry32{}))
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return matches, nil
}

func queryProcessCommandLine(processID uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, processID)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	var required uint32
	status, _, _ := procNtQueryInformationProcess.Call(
		uintptr(handle),
		processCommandLineInformation,
		0,
		0,
		uintptr(unsafe.Pointer(&required)),
	)
	if required == 0 {
		return "", fmt.Errorf("query process command line size failed: NTSTATUS 0x%08x", uint32(status))
	}
	buffer := make([]byte, required)
	status, _, _ = procNtQueryInformationProcess.Call(
		uintptr(handle),
		processCommandLineInformation,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unsafe.Pointer(&required)),
	)
	if int32(status) < 0 {
		return "", fmt.Errorf("query process command line failed: NTSTATUS 0x%08x", uint32(status))
	}
	value := (*nativeUnicodeString)(unsafe.Pointer(&buffer[0]))
	if value.Buffer == nil || value.Length == 0 {
		return "", nil
	}
	return windows.UTF16ToString(unsafe.Slice(value.Buffer, int(value.Length/2))), nil
}

func normalizeCommandLinePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if absolute, err := filepath.Abs(value); err == nil {
		value = absolute
	}
	return strings.ToLower(strings.TrimRight(filepath.Clean(value), `\/`))
}

func commandLineUsesWebViewProfile(commandLine, target, targetWithSuffix string) bool {
	args, err := windows.DecomposeCommandLine(commandLine)
	if err != nil {
		return false
	}
	const option = "--user-data-dir"
	for index := 0; index < len(args); index++ {
		arg := args[index]
		value := ""
		switch {
		case strings.EqualFold(arg, option) && index+1 < len(args):
			value = args[index+1]
		case len(arg) > len(option) && strings.EqualFold(arg[:len(option)], option) && arg[len(option)] == '=':
			value = arg[len(option)+1:]
		default:
			continue
		}
		normalized := normalizeCommandLinePath(value)
		return normalized == target || normalized == targetWithSuffix
	}
	return false
}
