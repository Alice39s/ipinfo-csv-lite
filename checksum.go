package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// runChecksum writes data/checksums.txt with the SHA-256 of every release
// artifact, in the standard `sha256sum` output format.
func runChecksum() error {
	return writeChecksums(dataDir)
}

// writeChecksums hashes every ipinfo-lite.* artifact in dir and writes the
// result to <dir>/checksums.txt in `sha256sum` format.
func writeChecksums(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "checksums.txt" {
			continue
		}
		// Only hash release artifacts, not intermediate files.
		if strings.HasPrefix(e.Name(), "ipinfo-lite.") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("no release artifacts found in %s", dir)
	}

	var sb strings.Builder
	for _, name := range names {
		sum, err := sha256File(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&sb, "%s  %s\n", sum, name)
	}

	output := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(output, []byte(sb.String()), 0o644); err != nil {
		return err
	}

	fmt.Printf("Wrote %s (%d artifacts)\n", output, len(names))
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
