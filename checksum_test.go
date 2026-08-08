package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteChecksums(t *testing.T) {
	dir := t.TempDir()

	artifacts := map[string]string{
		"ipinfo-lite.csv":      "csv-content",
		"ipinfo-lite.csv.gz":   "gz-content",
		"ipinfo-lite.ipv4.xdb": "xdb-content",
		"ipinfo_lite.csv":      "source, must be excluded",
		"ipinfo_lite.mmdb":     "source, must be excluded",
		"ipinfo.version":       "intermediate, must be excluded",
	}
	for name, content := range artifacts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := writeChecksums(dir); err != nil {
		t.Fatalf("writeChecksums: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		t.Fatalf("read checksums.txt: %v", err)
	}
	out := string(data)

	for _, name := range []string{"ipinfo-lite.csv", "ipinfo-lite.csv.gz", "ipinfo-lite.ipv4.xdb"} {
		if !strings.Contains(out, "  "+name+"\n") {
			t.Errorf("checksums.txt missing entry for %s", name)
		}
	}
	for _, name := range []string{"ipinfo_lite.csv", "ipinfo_lite.mmdb", "ipinfo.version"} {
		if strings.Contains(out, name) {
			t.Errorf("checksums.txt should not contain %s", name)
		}
	}

	// Verify one hash end-to-end.
	sum, err := sha256File(filepath.Join(dir, "ipinfo-lite.csv"))
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	if !strings.HasPrefix(out, sum+"  ipinfo-lite.csv\n") {
		t.Errorf("checksums.txt does not start with the expected hash line")
	}
}

func TestWriteChecksumsEmptyDir(t *testing.T) {
	if err := writeChecksums(t.TempDir()); err == nil {
		t.Error("expected error for directory without artifacts")
	}
}
