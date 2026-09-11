package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestEmbeddedDefaultConditionIcons(t *testing.T) {
	staticRoot, err := fs.Sub(staticFiles, embeddedStaticDir)
	if err != nil {
		t.Fatal(err)
	}
	defaultCCIDs := []int{
		63, 177, 192, 193, 406, 407, 408, 476, 479,
		511, 512, 513, 514, 515, 612, 645, 680, 717,
		796, 800, 874, 887, 891, 914, 915, 934, 938,
		975, 978, 1016, 1023, 1033, 1035, 1036, 1037,
		1045, 1046, 1120, 1123, 1144, 1145, 1146, 1150,
		1159, 1161, 1225,
	}
	for _, ccID := range defaultCCIDs {
		path := fmt.Sprintf("condition-icons/%d.png", ccID)
		info, err := fs.Stat(staticRoot, path)
		if err != nil {
			t.Fatalf("default Buff CC %d is not embedded: %v", ccID, err)
		}
		if info.Size() == 0 {
			t.Fatalf("embedded Buff CC %d icon is empty", ccID)
		}
	}
}

func TestBundledConditionIconPath(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/res/characterconditionimage/cn/680/680.png", "/condition-icons/680.png", true},
		{"/res/characterconditionimage/kr/680/680.png", "", false},
		{"/res/characterconditionimage/cn/680/681.png", "", false},
		{"/res/characterconditionimage/cn/../680.png", "", false},
	}
	for _, test := range tests {
		got, ok := bundledConditionIconPath(test.path)
		if got != test.want || ok != test.ok {
			t.Fatalf("bundledConditionIconPath(%q) = %q, %v; want %q, %v", test.path, got, ok, test.want, test.ok)
		}
	}
}

func TestLocalFirstResourceHandler(t *testing.T) {
	local := fstest.MapFS{
		"condition-icons/680.png": &fstest.MapFile{Data: []byte("bundled-icon")},
	}
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		_, _ = w.Write([]byte("upstream"))
	})
	handler := localFirstResourceHandler(fs.FS(local), upstream)

	localResponse := httptest.NewRecorder()
	handler.ServeHTTP(localResponse, httptest.NewRequest(http.MethodGet, "/res/characterconditionimage/cn/680/680.png", nil))
	if got := localResponse.Body.String(); got != "bundled-icon" {
		t.Fatalf("bundled response = %q; want bundled-icon", got)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream called %d times for bundled icon", upstreamCalls)
	}

	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/res/characterconditionimage/cn/999/999.png", nil))
	if got := missingResponse.Body.String(); got != "upstream" {
		t.Fatalf("fallback response = %q; want upstream", got)
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstream called %d times after missing icon; want 1", upstreamCalls)
	}
}
