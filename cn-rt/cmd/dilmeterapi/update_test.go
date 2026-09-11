package main

import (
	"net/url"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct{ a, b string; want int }{{"1.3.2", "1.3.1", 1}, {"1.2.35", "1.2.35", 0}, {"1.2.9", "1.2.10", -1}} {
		if got := compareVersions(test.a, test.b); got != test.want { t.Fatalf("compareVersions(%q,%q)=%d want %d", test.a, test.b, got, test.want) }
	}
}

func TestValidateUpdateManifestRequiresHTTPSAndHash(t *testing.T) {
	originalVariant := BuildVariant
	t.Cleanup(func() { BuildVariant = originalVariant })
	BuildVariant = "release"
	valid := updateManifest{Version: "1.3.2", URL: "https://example.com/DilmeterCN.zip", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := validateUpdateManifest(valid); err != nil { t.Fatal(err) }
	valid.URL = "http://example.com/update.zip"
	if err := validateUpdateManifest(valid); err == nil { t.Fatal("HTTP update URL was accepted") }
}

func TestLocalUpdateTestAllowsOnlyLoopbackHTTP(t *testing.T) {
	originalVariant := BuildVariant
	t.Cleanup(func() { BuildVariant = originalVariant })
	BuildVariant = "local-update-test"
	for _, raw := range []string{"http://127.0.0.1:18080/update.zip", "http://localhost/update.zip", "http://[::1]:18080/update.zip"} {
		candidate, err := url.Parse(raw)
		if err != nil { t.Fatal(err) }
		if err := validateUpdateTransportURL(candidate); err != nil { t.Fatalf("%s: %v", raw, err) }
	}
	for _, raw := range []string{"http://example.com/update.zip", "http://192.168.1.2/update.zip"} {
		candidate, _ := url.Parse(raw)
		if err := validateUpdateTransportURL(candidate); err == nil { t.Fatalf("remote HTTP accepted: %s", raw) }
	}
	candidate, _ := url.Parse("http://user:password@127.0.0.1:18080/update.zip")
	if err := validateUpdateTransportURL(candidate); err == nil {
		t.Fatal("credential-bearing loopback URL was accepted")
	}
}

func TestReleaseIgnoresLocalUpdateBaseURL(t *testing.T) {
	originalVariant, originalBase := BuildVariant, UpdateBaseURL
	t.Cleanup(func() { BuildVariant, UpdateBaseURL = originalVariant, originalBase })
	UpdateBaseURL = "http://127.0.0.1:18080"
	BuildVariant = "release"
	if got := updateManifestEndpoint("DilmeterCN"); len(got) < len(officialUpdateBaseURL) || got[:len(officialUpdateBaseURL)] != officialUpdateBaseURL {
		t.Fatalf("release endpoint used override: %s", got)
	}
	BuildVariant = "local-update-test"
	if got := updateManifestEndpoint("DilmeterCN"); got[:len("http://127.0.0.1:18080")] != "http://127.0.0.1:18080" {
		t.Fatalf("local test endpoint ignored override: %s", got)
	}
	BuildVariant = "local-update-test-debug"
	if got := updateManifestEndpoint("DilmeterCN"); len(got) < len(officialUpdateBaseURL) || got[:len(officialUpdateBaseURL)] != officialUpdateBaseURL {
		t.Fatalf("near-match build variant used override: %s", got)
	}
}

func TestReleaseRejectsLoopbackHTTP(t *testing.T) {
	originalVariant := BuildVariant
	t.Cleanup(func() { BuildVariant = originalVariant })
	BuildVariant = "release"
	candidate, err := url.Parse("http://127.0.0.1:18080/update.zip")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateTransportURL(candidate); err == nil {
		t.Fatal("release build accepted loopback HTTP")
	}
}
