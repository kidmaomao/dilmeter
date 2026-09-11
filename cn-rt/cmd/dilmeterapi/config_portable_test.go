package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyMissingTreePreservesPortableData(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "logs", "old.ndjson"), []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.yaml"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "config.yaml"), []byte("portable"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyMissingTree(source, target); err != nil {
		t.Fatal(err)
	}
	logData, err := os.ReadFile(filepath.Join(target, "logs", "old.ndjson"))
	if err != nil || string(logData) != "legacy" {
		t.Fatalf("legacy log was not copied: %q, %v", logData, err)
	}
	configData, err := os.ReadFile(filepath.Join(target, "config.yaml"))
	if err != nil || string(configData) != "portable" {
		t.Fatalf("portable config was overwritten: %q, %v", configData, err)
	}
}
