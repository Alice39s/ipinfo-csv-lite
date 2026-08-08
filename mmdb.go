package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

// runMmdb validates and publishes IPinfo's official Lite MMDB without changing
// its records, metadata, or byte representation.
func runMmdb() error {
	source := filepath.Join(dataDir, sourceMMDBName)
	output := filepath.Join(dataDir, "ipinfo-lite.mmdb")
	if err := publishOfficialMmdb(source, output); err != nil {
		return err
	}
	info, err := os.Stat(output)
	if err != nil {
		return err
	}
	fmt.Printf("Published official IPinfo Lite MMDB unchanged: %s (%d bytes)\n", output, info.Size())
	return nil
}

func publishOfficialMmdb(source, destination string) error {
	database, err := maxminddb.Open(source)
	if err != nil {
		return fmt.Errorf("validate official MMDB: %w", err)
	}
	metadata := database.Metadata
	if err := database.Close(); err != nil {
		return fmt.Errorf("close official MMDB: %w", err)
	}
	databaseType := strings.ToLower(metadata.DatabaseType)
	if metadata.BinaryFormatMajorVersion != 2 || metadata.IPVersion != 6 ||
		!strings.Contains(databaseType, "ipinfo") || !strings.Contains(databaseType, "lite") {
		return fmt.Errorf(
			"validate official MMDB: unexpected metadata (type %q, format %d, IP version %d)",
			metadata.DatabaseType,
			metadata.BinaryFormatMajorVersion,
			metadata.IPVersion,
		)
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(temp, input); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return err
	}
	return nil
}
