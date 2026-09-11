//go:build !windows

package util

import "os"

// windows의 경우에는 git bash에서 실행시 문제가 있기 때문에 os filter 사용

func IsOpenedStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		logger.Println("os.Stdin.Stat failed:", err)
		return false
	}

	mode := fi.Mode()

	// terminal
	if (mode & os.ModeCharDevice) != 0 {
		return false
	}

	return true
}
