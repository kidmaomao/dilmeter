package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateFallbackTransport func(*http.Request) (*http.Response, error)

func (f updateFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestUpdateManifestPublicMirrorFallback(t *testing.T) {
	const valid = `{"version":"1.4.2","url":"https://gear.noginogi.sbs/downloads/DilmeterCN.zip","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	for _, tc := range []struct {
		name         string
		status       int
		body         string
		transportErr error
		mirrorStatus int
		wantRequests int
		wantErr      bool
	}{
		{"github available", http.StatusOK, valid, nil, http.StatusOK, 1, false},
		{"private repository", http.StatusNotFound, "", nil, http.StatusOK, 2, false},
		{"invalid primary data", http.StatusOK, "invalid", nil, http.StatusOK, 2, false},
		{"primary timed out", 0, "", context.DeadlineExceeded, http.StatusOK, 2, false},
		{"both unavailable", http.StatusNotFound, "", nil, http.StatusServiceUnavailable, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalClient := updateHTTPClient
			t.Cleanup(func() { updateHTTPClient = originalClient })
			requests := 0
			updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				status, body := tc.status, tc.body
				switch requests {
				case 1:
					if !strings.HasPrefix(req.URL.String(), officialUpdateBaseURL+"/") {
						t.Fatalf("unexpected primary endpoint: %s", req.URL)
					}
					if tc.transportErr != nil {
						return nil, tc.transportErr
					}
				case 2:
					if !strings.HasPrefix(req.URL.String(), mirrorUpdateBaseURL+"/"+AppName+".json?") {
						t.Fatalf("unexpected mirror endpoint: %s", req.URL)
					}
					status, body = tc.mirrorStatus, valid
				default:
					t.Fatal("unexpected extra request")
				}
				if _, ok := req.Context().Deadline(); !ok {
					t.Fatal("manifest request has no timeout")
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			manifest, err := fetchUpdateManifest(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if requests != tc.wantRequests {
				t.Fatalf("requests = %d, want %d", requests, tc.wantRequests)
			}
			if !tc.wantErr && manifest.Version != "1.4.2" {
				t.Fatalf("unexpected manifest: %+v", manifest)
			}
		})
	}
}

func TestUpdateManifestDoesNotFallbackAfterCancellation(t *testing.T) {
	originalClient := updateHTTPClient
	t.Cleanup(func() { updateHTTPClient = originalClient })
	requests := 0
	updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, req.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchUpdateManifest(ctx); err == nil {
		t.Fatal("cancelled request succeeded")
	}
	if requests > 1 {
		t.Fatalf("cancelled request contacted mirror: %d requests", requests)
	}
}

func TestUpdateManifestDoesNotFallbackForTestEndpoint(t *testing.T) {
	originalClient, originalEndpoint := updateHTTPClient, updateManifestEndpoint
	t.Cleanup(func() { updateHTTPClient, updateManifestEndpoint = originalClient, originalEndpoint })
	updateManifestEndpoint = func(string) string { return "https://example.com/test.json" }
	requests := 0
	updateHTTPClient = &http.Client{Transport: updateFallbackTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("test endpoint unavailable")
	})}
	if _, err := fetchUpdateManifest(context.Background()); err == nil {
		t.Fatal("failed test endpoint succeeded")
	}
	if requests != 1 {
		t.Fatalf("test endpoint contacted production mirror: %d requests", requests)
	}
}
