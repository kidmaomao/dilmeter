//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOnlineUpdatePipelineDownloadsAndValidatesReleaseZip(t *testing.T) {
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	executable, err := zipWriter.Create("DilmeterCN-v1.3.2.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executable.Write([]byte("test executable")); err != nil {
		t.Fatal(err)
	}
	// Reproduce ZIPs made by Windows tar.exe: the filename is CP936/GBK,
	// and the UTF-8 general-purpose bit is not set.
	helpHeader := &zip.FileHeader{
		Name:    string([]byte{0xCA, 0xB9, 0xD3, 0xC3, 0xCB, 0xB5, 0xC3, 0xF7, '.', 'm', 'd'}),
		Method:  zip.Deflate,
		NonUTF8: true,
	}
	help, err := zipWriter.CreateHeader(helpHeader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := help.Write([]byte("test help")); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(archive.Bytes())

	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads/DilmeterCN.json":
			_ = json.NewEncoder(w).Encode(updateManifest{
				Version: "1.3.2",
				URL:     server.URL + "/downloads/DilmeterCN.zip",
				SHA256:  hex.EncodeToString(hash[:]),
				Size:    int64(archive.Len()),
				Notes:   "integration test",
			})
		case "/downloads/DilmeterCN.zip":
			_, _ = w.Write(archive.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalClient := updateHTTPClient
	originalEndpoint := updateManifestEndpoint
	originalDirectory := updateDownloadDirectory
	originalName := AppName
	originalVersion := AppVersion
	defer func() {
		updateHTTPClient = originalClient
		updateManifestEndpoint = originalEndpoint
		updateDownloadDirectory = originalDirectory
		AppName = originalName
		AppVersion = originalVersion
	}()

	downloadDir := t.TempDir()
	updateHTTPClient = server.Client()
	updateManifestEndpoint = func(string) string { return server.URL + "/downloads/DilmeterCN.json" }
	updateDownloadDirectory = func() (string, error) { return downloadDir, nil }
	AppName = "DilmeterCN"
	AppVersion = "1.3.0"

	manifest, err := fetchUpdateManifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status := toUpdateStatus(manifest); !status.Available || status.LatestVersion != "1.3.2" {
		t.Fatalf("unexpected update status: %+v", status)
	}
	downloaded, err := downloadVerifiedUpdate(context.Background(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(downloaded) != "DilmeterCN-1.3.2.zip" {
		t.Fatalf("unexpected downloaded filename: %s", downloaded)
	}
	if err := verifyFileSHA256(downloaded, manifest.SHA256); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(t.TempDir(), "stage")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	files, err := extractVerifiedUpdateZip(downloaded, stage)
	if err != nil {
		t.Fatal(err)
	}
	if executable, err := chooseUpdateExecutable(files, AppName); err != nil || executable != "DilmeterCN-v1.3.2.exe" {
		t.Fatalf("update executable = %q, %v", executable, err)
	}
	if _, err := os.Stat(filepath.Join(stage, "使用说明.md")); err != nil {
		t.Fatalf("legacy GBK filename was not decoded to 使用说明.md: %v (files: %q)", err, files)
	}
}
