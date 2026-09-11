package main

import "testing"

func TestResourcePackRegionRewrite(t *testing.T) {
	if got := resourcePackRelativeURLPath("resourcedata/cn/cn_resourcedata.bin.br", "tw"); got != "resourcedata/tw/tw_resourcedata.bin.br" {
		t.Fatalf("TW data path = %q", got)
	}
	if got := resourcePackRelativeURLPath("resourceversion/cn/cn_resourceversion.json", "us"); got != "resourceversion/us/us_resourceversion.json" {
		t.Fatalf("US version path = %q", got)
	}
	if got := resourcePackRelativeURLPath("resourcedata/cn/cn_resourcedata.bin.br", "../../bad"); got != "resourcedata/cn/cn_resourcedata.bin.br" {
		t.Fatalf("invalid region changed path to %q", got)
	}
	for _, region := range []string{"kr", "krt", "cn", "jp", "tw", "us"} {
		if !validResourcePackRegion(region) {
			t.Fatalf("supported region %q was rejected", region)
		}
	}
}
