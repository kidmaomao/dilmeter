package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxUpdateDownloadBytes int64 = 500 << 20
const officialUpdateBaseURL = "https://github.com/kidmaomao/dilmeter/releases/latest/download"

// Used only by builds whose BuildVariant is exactly local-update-test.
var UpdateBaseURL = ""

type updateManifest struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type updateStatus struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	Notes          string `json:"notes,omitempty"`
	SavedPath      string `json:"savedPath,omitempty"`
	Installing     bool   `json:"installing,omitempty"`
}

var updateHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return fmt.Errorf("too many redirects")
		}
		if err := validateUpdateTransportURL(req.URL); err != nil {
			return fmt.Errorf("unsafe update redirect: %w", err)
		}
		return nil
	},
}

var updateManifestEndpoint = func(appName string) string {
	baseURL := officialUpdateBaseURL
	if BuildVariant == "local-update-test" && strings.TrimSpace(UpdateBaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(UpdateBaseURL), "/")
	}
	return fmt.Sprintf("%s/%s.json?check=%d", baseURL, url.PathEscape(appName), time.Now().Unix())
}

var updateDownloadDirectory = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		manifest, err := fetchUpdateManifest(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("检查更新失败：%v", err), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(toUpdateStatus(manifest))
	case http.MethodPost:
		if r.Header.Get("X-Dilmeter-Action") != "install-update" {
			http.Error(w, "missing update confirmation", http.StatusForbidden)
			return
		}
		manifest, err := fetchUpdateManifest(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("检查更新失败：%v", err), http.StatusBadGateway)
			return
		}
		status := toUpdateStatus(manifest)
		if !status.Available {
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		path, err := downloadVerifiedUpdate(r.Context(), manifest)
		if err != nil {
			http.Error(w, fmt.Sprintf("下载更新失败：%v", err), http.StatusBadGateway)
			return
		}
		status.SavedPath = path
		if err := startVerifiedUpdateInstall(path, manifest); err != nil {
			http.Error(w, fmt.Sprintf("启动更新失败：%v", err), http.StatusInternalServerError)
			return
		}
		status.Installing = true
		_ = json.NewEncoder(w).Encode(status)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func fetchUpdateManifest(ctx context.Context) (updateManifest, error) {
	manifestURL := updateManifestEndpoint(AppName)
	parsedManifestURL, err := url.Parse(manifestURL)
	if err != nil {
		return updateManifest{}, err
	}
	if err := validateUpdateTransportURL(parsedManifestURL); err != nil {
		return updateManifest{}, fmt.Errorf("invalid update manifest endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return updateManifest{}, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s updater", AppName, AppVersion))
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	response, err := updateHTTPClient.Do(req)
	if err != nil {
		return updateManifest{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return updateManifest{}, fmt.Errorf("更新服务返回 HTTP %d", response.StatusCode)
	}
	var manifest updateManifest
	if err := json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&manifest); err != nil {
		return manifest, err
	}
	if err := validateUpdateManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validateUpdateManifest(manifest updateManifest) error {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`).MatchString(strings.TrimSpace(manifest.Version)) {
		return fmt.Errorf("版本号格式无效")
	}
	downloadURL, err := url.Parse(strings.TrimSpace(manifest.URL))
	if err != nil || validateUpdateTransportURL(downloadURL) != nil {
		return fmt.Errorf("下载地址必须使用 HTTPS")
	}
	hash := strings.ToLower(strings.TrimSpace(manifest.SHA256))
	if len(hash) != 64 {
		return fmt.Errorf("更新清单缺少 SHA-256")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return fmt.Errorf("SHA-256 格式无效")
	}
	if manifest.Size < 0 || manifest.Size > maxUpdateDownloadBytes {
		return fmt.Errorf("更新包大小无效")
	}
	return nil
}

func toUpdateStatus(manifest updateManifest) updateStatus {
	return updateStatus{CurrentVersion: AppVersion, LatestVersion: manifest.Version, Available: compareVersions(manifest.Version, AppVersion) > 0, Notes: manifest.Notes}
}

func compareVersions(left, right string) int {
	parts := func(version string) []int {
		version = strings.SplitN(strings.SplitN(version, "-", 2)[0], "+", 2)[0]
		values := []int{0, 0, 0}
		for index, raw := range strings.Split(version, ".") {
			if index >= len(values) {
				break
			}
			values[index], _ = strconv.Atoi(raw)
		}
		return values
	}
	a, b := parts(left), parts(right)
	for index := range a {
		if a[index] > b[index] {
			return 1
		}
		if a[index] < b[index] {
			return -1
		}
	}
	return 0
}

func downloadVerifiedUpdate(ctx context.Context, manifest updateManifest) (string, error) {
	downloadURL, err := url.Parse(strings.TrimSpace(manifest.URL))
	if err != nil {
		return "", err
	}
	if err := validateUpdateTransportURL(downloadURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s updater", AppName, AppVersion))
	response, err := updateHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载地址返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxUpdateDownloadBytes {
		return "", fmt.Errorf("更新包超过大小限制")
	}
	downloads, err := updateDownloadDirectory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(downloads, 0755); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(downloads, ".dilmeter-update-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(response.Body, maxUpdateDownloadBytes+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written > maxUpdateDownloadBytes {
		return "", fmt.Errorf("更新包超过大小限制")
	}
	if manifest.Size > 0 && written != manifest.Size {
		return "", fmt.Errorf("更新包大小不符")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), manifest.SHA256) {
		return "", fmt.Errorf("更新包 SHA-256 校验失败")
	}
	finalPath := filepath.Join(downloads, fmt.Sprintf("%s-%s.zip", AppName, manifest.Version))
	_ = os.Remove(finalPath)
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}

func validateUpdateTransportURL(candidate *url.URL) error {
	if candidate == nil || candidate.Host == "" || candidate.User != nil {
		return fmt.Errorf("update URL must contain a host and no credentials")
	}
	if strings.EqualFold(candidate.Scheme, "https") {
		return nil
	}
	if candidate.Scheme != "http" || BuildVariant != "local-update-test" || !isLoopbackUpdateHost(candidate.Hostname()) {
		return fmt.Errorf("update URL must use HTTPS; loopback HTTP is allowed only for local-update-test builds")
	}
	return nil
}

func isLoopbackUpdateHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	address := net.ParseIP(strings.TrimSpace(host))
	return address != nil && address.IsLoopback()
}
