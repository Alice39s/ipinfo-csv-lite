package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const downloadURL = "https://ipinfo.io/data/ipinfo_lite.csv.gz"

// runUpdate downloads ipinfo_lite.csv.gz and extracts it to data/country_asn.csv,
// skipping the download if today's data was already fetched (tracked via
// data/ipinfo.version, which stores an ISO date).
func runUpdate() error {
	token := os.Getenv("IPINFO_TOKEN")
	if token == "" {
		return fmt.Errorf("IPINFO_TOKEN environment variable is not set")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	today := time.Now().Format("2006-01-02")
	versionFile := filepath.Join(dataDir, "ipinfo.version")
	if stored, err := os.ReadFile(versionFile); err == nil && string(bytes.TrimSpace(stored)) == today {
		fmt.Printf("Database is already up to date for %s\n", today)
		return nil
	}

	tmpFile := filepath.Join(dataDir, "country_asn.csv.gz")
	csvFile := filepath.Join(dataDir, "country_asn.csv")
	defer os.Remove(tmpFile)

	fmt.Println("Downloading database...")
	req, err := http.NewRequest(http.MethodGet, downloadURL+"?token="+token, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ipinfo-csv-lite/"+version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// *url.Error would embed the request URL (which carries the token);
		// unwrap it so the token never lands in logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("download failed: %w", urlErr.Err)
		}
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	if err := out.Close(); err != nil {
		return err
	}

	fmt.Println("Extracting database...")
	if err := gunzipFile(tmpFile, csvFile); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	if err := os.WriteFile(versionFile, []byte(today), 0o644); err != nil {
		return err
	}

	fmt.Println("Database updated and extracted successfully")
	return nil
}

func gunzipFile(src, dst string) error {
	fIn, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fIn.Close()

	zr, err := gzip.NewReader(fIn)
	if err != nil {
		return err
	}
	defer zr.Close()

	fOut, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(fOut, zr); err != nil {
		fOut.Close()
		return err
	}
	return fOut.Close()
}
