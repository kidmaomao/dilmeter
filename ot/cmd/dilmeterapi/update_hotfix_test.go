package main

import "testing"

func TestOTHotfixUpdateCompatibility(t *testing.T) {
	oldVersion, oldDisplay := AppVersion, AppDisplayVersion
	t.Cleanup(func() { AppVersion, AppDisplayVersion = oldVersion, oldDisplay })
	manifest := updateManifest{Version: "1.5.3", DisplayVersion: "1.5.2 R2", Notes: "新增繁体中文及台服档适配"}
	// This numeric comparison is also the one shipped in the old 1.5.2 client.
	if compareVersions(manifest.Version, "1.5.2") <= 0 {
		t.Fatal("legacy OT 1.5.2 must receive the hotfix")
	}
	AppVersion, AppDisplayVersion = "1.5.2", "1.5.2"
	if status := toUpdateStatus(manifest); !status.Available || status.LatestVersion != "1.5.2 R2" {
		t.Fatalf("hotfix offer: %+v", status)
	}
	AppVersion, AppDisplayVersion = "1.5.3", "1.5.2 R2"
	if status := toUpdateStatus(manifest); status.Available || status.CurrentVersion != "1.5.2 R2" {
		t.Fatalf("installed hotfix must not repeatedly offer itself: %+v", status)
	}
	if toUpdateStatus(updateManifest{Version: "1.5.2"}).Available {
		t.Fatal("stale mirror must not downgrade the hotfix")
	}
	if status := toUpdateStatus(updateManifest{Version: "1.5.4"}); !status.Available || status.LatestVersion != "1.5.4" {
		t.Fatalf("next release must remain reachable: %+v", status)
	}
}
