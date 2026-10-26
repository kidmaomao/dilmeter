package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGiteePreferredManifestAndUnknownSource(t *testing.T) {
	original := updateHTTPClient
	t.Cleanup(func() { updateHTTPClient = original })
	var contacted []string
	updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
		contacted = append(contacted, req.URL.Host)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"version":"1.6.4","url":"https://gitee.com/release.zip","size":10,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)), Header: make(http.Header)}, nil
	})}
	manifest, err := fetchUpdateManifestWithPreference(context.Background(), "gitee")
	if err != nil || manifest.Source != "gitee" || len(contacted) != 1 || contacted[0] != "gitee.com" {
		t.Fatalf("Gitee preference was ignored: %+v %v %v", manifest, err, contacted)
	}
	w := httptest.NewRecorder()
	handleUpdate(w, httptest.NewRequest("GET", "/api/update?source=arbitrary-server", nil))
	if w.Code != http.StatusBadRequest || len(contacted) != 1 {
		t.Fatalf("invalid source reached the network: %d %v", w.Code, contacted)
	}
}

type stalledUpdateBody struct{ ctx context.Context }

func (body stalledUpdateBody) Read([]byte) (int, error) { <-body.ctx.Done(); return 0, body.ctx.Err() }
func (body stalledUpdateBody) Close() error             { return nil }

func TestDownloadFallbackVerifiesIdentityAndCleansPartialFiles(t *testing.T) {
	for _, mode := range []string{"unavailable", "stalled body", "stalled headers", "corrupt bytes", "stale Gitee", "wrong hash", "wrong size"} {
		t.Run(mode, func(t *testing.T) {
			client, directory, window, minimum := updateHTTPClient, updateDownloadDirectory, updateDownloadWindow, updateDownloadMinBytes
			t.Cleanup(func() {
				updateHTTPClient, updateDownloadDirectory, updateDownloadWindow, updateDownloadMinBytes = client, directory, window, minimum
			})
			root := t.TempDir()
			updateDownloadDirectory = func() (string, error) { return root, nil }
			updateDownloadWindow, updateDownloadMinBytes = 20*time.Millisecond, 1
			payload := "verified release bytes"
			hash := sha256.Sum256([]byte(payload))
			selected := updateManifest{Version: "1.6.4", URL: "https://github.com/package.zip", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(payload)), Source: "github"}
			var downloaded []string
			updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
				body, status := payload, 200
				if strings.HasSuffix(req.URL.Path, ".json") {
					candidate := selected
					candidate.URL = "https://" + req.URL.Host + "/package.zip"
					if req.URL.Host == "gitee.com" {
						switch mode {
						case "stale Gitee":
							candidate.Version = "1.6.3"
						case "wrong hash":
							candidate.SHA256 = strings.Repeat("a", 64)
						case "wrong size":
							candidate.Size++
						}
					}
					encoded, _ := json.Marshal(candidate)
					body = string(encoded)
				} else {
					downloaded = append(downloaded, req.URL.Host)
					if req.URL.Host == "github.com" {
						switch mode {
						case "stalled body":
							return &http.Response{StatusCode: 200, Body: stalledUpdateBody{req.Context()}, Header: make(http.Header)}, nil
						case "stalled headers":
							<-req.Context().Done()
							return nil, req.Context().Err()
						case "corrupt bytes":
							body = "corrupt release bytes!"
						default:
							status = http.StatusServiceUnavailable
						}
					}
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			path, source, err := downloadUpdateWithFallback(context.Background(), selected, "auto")
			want := "gitee"
			if mode == "stale Gitee" || mode == "wrong hash" || mode == "wrong size" {
				want = "mirror"
			}
			if err != nil || source != want {
				t.Fatalf("fallback: %q %v, expected %s", source, err, want)
			}
			if len(downloaded) != 2 {
				t.Fatalf("mismatched package was downloaded: %v", downloaded)
			}
			bytes, err := os.ReadFile(path)
			if err != nil || string(bytes) != payload {
				t.Fatalf("invalid saved package: %v", err)
			}
			files, _ := os.ReadDir(root)
			if len(files) != 1 || strings.HasSuffix(files[0].Name(), ".tmp") {
				t.Fatalf("partial download leaked: %v", files)
			}
		})
	}
}

func TestCancelledUpdateDoesNotTryAnotherDownload(t *testing.T) {
	original := updateHTTPClient
	t.Cleanup(func() { updateHTTPClient = original })
	ctx, cancel := context.WithCancel(context.Background())
	count := 0
	updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
		count++
		cancel()
		return nil, context.Canceled
	})}
	_, _, err := downloadUpdateWithFallback(ctx, updateManifest{URL: "https://github.com/package.zip", Source: "github"}, "auto")
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("cancelled download retried: %d %v", count, err)
	}
}
