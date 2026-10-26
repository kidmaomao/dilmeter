package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const giteeUpdateBaseURL = "https://gitee.com/kidmao100/dilmeter-releases/raw/master/latest"

type updateSource struct {
	ID, Name, ManifestURL string
}

var updateDownloadWindow = 10 * time.Second
var updateDownloadMinBytes int64 = 320 << 10
var errSlowUpdateSource = errors.New("连接超时或下载持续过慢，尝试备用来源")

func validUpdateSourcePreference(value string) bool {
	return value == "" || value == "auto" || value == "github" || value == "gitee"
}

func orderedUpdateSources(preference string) []updateSource {
	primary := updateManifestEndpoint(AppName)
	github := updateSource{"github", "GitHub", primary}
	// Integration tests and local-update-test builds must never reach production mirrors.
	if !strings.HasPrefix(primary, officialUpdateBaseURL+"/") {
		return []updateSource{{"local", "测试来源", primary}}
	}
	endpoint := func(base string) string {
		return fmt.Sprintf("%s/%s.json?check=%d", base, url.PathEscape(AppName), time.Now().Unix())
	}
	gitee := updateSource{"gitee", "Gitee", endpoint(giteeUpdateBaseURL)}
	mirror := updateSource{"mirror", "下载站", endpoint(mirrorUpdateBaseURL)}
	if preference == "gitee" {
		return []updateSource{gitee, github, mirror}
	}
	return []updateSource{github, gitee, mirror}
}

func fetchUpdateManifestFromSource(ctx context.Context, source updateSource) (updateManifest, error) {
	manifest, err := fetchUpdateManifestURL(ctx, source.ManifestURL)
	manifest.Source = source.ID
	return manifest, err
}

func fetchUpdateManifestWithPreference(ctx context.Context, preference string) (updateManifest, error) {
	var failures []string
	for _, source := range orderedUpdateSources(preference) {
		if err := ctx.Err(); err != nil {
			return updateManifest{}, err
		}
		manifest, err := fetchUpdateManifestFromSource(ctx, source)
		if err == nil {
			return manifest, nil
		}
		failures = append(failures, source.Name+": "+err.Error())
	}
	return updateManifest{}, errors.New(strings.Join(failures, "; "))
}

type updateDownloadProgress struct{ written atomic.Int64 }

func (progress *updateDownloadProgress) Write(data []byte) (int, error) {
	progress.written.Add(int64(len(data)))
	return len(data), nil
}

func downloadUpdateWithSpeedCheck(ctx context.Context, manifest updateManifest) (string, error) {
	attempt, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	progress := &updateDownloadProgress{}
	stopped := make(chan struct{})
	defer close(stopped)
	window, minimum := updateDownloadWindow, updateDownloadMinBytes
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		var previous int64
		for {
			select {
			case <-stopped:
				return
			case <-attempt.Done():
				return
			case <-ticker.C:
				current := progress.written.Load()
				if current-previous < minimum {
					cancel(errSlowUpdateSource)
					return
				}
				previous = current
			}
		}
	}()
	path, err := downloadVerifiedUpdateWithProgress(attempt, manifest, progress)
	if err != nil && errors.Is(context.Cause(attempt), errSlowUpdateSource) {
		return "", errSlowUpdateSource
	}
	return path, err
}

func sameUpdateArtifact(left, right updateManifest) bool {
	return left.Version == right.Version && left.Size == right.Size && strings.EqualFold(left.SHA256, right.SHA256)
}

func downloadUpdateWithFallback(ctx context.Context, selected updateManifest, preference string) (string, string, error) {
	path, err := downloadUpdateWithSpeedCheck(ctx, selected)
	if err == nil {
		return path, selected.Source, nil
	}
	failures := []string{selected.Source + ": " + err.Error()}
	for _, source := range orderedUpdateSources(preference) {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		if source.ID == selected.Source {
			continue
		}
		candidate, fetchErr := fetchUpdateManifestFromSource(ctx, source)
		if fetchErr != nil {
			failures = append(failures, source.Name+": "+fetchErr.Error())
			continue
		}
		// Never install another version or build while recovering a failed download.
		if !sameUpdateArtifact(selected, candidate) {
			failures = append(failures, source.Name+": 版本或文件校验信息尚未同步")
			continue
		}
		path, downloadErr := downloadUpdateWithSpeedCheck(ctx, candidate)
		if downloadErr == nil {
			return path, source.ID, nil
		}
		failures = append(failures, source.Name+": "+downloadErr.Error())
	}
	return "", "", errors.New(strings.Join(failures, "; "))
}
