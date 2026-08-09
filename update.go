package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	liteCSVURL      = "https://ipinfo.io/data/ipinfo_lite.csv.gz"
	liteMMDBURL     = "https://ipinfo.io/data/ipinfo_lite.mmdb"
	sourceCSVName   = "ipinfo_lite.csv"
	sourceMMDBName  = "ipinfo_lite.mmdb"
	versionFileName = "ipinfo.version"
)

type updateConfig struct {
	client          *http.Client
	dataDir, token  string
	today           string
	csvURL, mmdbURL string
}

// runUpdate downloads the official IPinfo Lite CSV and MMDB, verifies both
// against IPinfo's SHA-256 endpoints, and updates the local date marker only
// after both source files are ready.
func runUpdate() error {
	token := os.Getenv("IPINFO_TOKEN")
	if token == "" {
		return fmt.Errorf("IPINFO_TOKEN environment variable is not set")
	}
	return updateDatabase(updateConfig{
		client:  &http.Client{Timeout: 10 * time.Minute},
		dataDir: dataDir,
		token:   token,
		today:   time.Now().Format("2006-01-02"),
		csvURL:  liteCSVURL,
		mmdbURL: liteMMDBURL,
	})
}

func updateDatabase(config updateConfig) error {
	if err := os.MkdirAll(config.dataDir, 0o755); err != nil {
		return err
	}
	versionPath := filepath.Join(config.dataDir, versionFileName)
	csvPath := filepath.Join(config.dataDir, sourceCSVName)
	mmdbPath := filepath.Join(config.dataDir, sourceMMDBName)
	if currentVersion(versionPath) == config.today && fileExists(csvPath) && fileExists(mmdbPath) {
		fmt.Printf("Database is already up to date for %s\n", config.today)
		return nil
	}

	csvGzipTemp, err := os.CreateTemp(config.dataDir, ".ipinfo-lite-csv-*.gz")
	if err != nil {
		return err
	}
	csvGzipPath := csvGzipTemp.Name()
	defer os.Remove(csvGzipPath)
	if err := csvGzipTemp.Close(); err != nil {
		return err
	}

	csvTemp, err := os.CreateTemp(config.dataDir, ".ipinfo-lite-csv-*.csv")
	if err != nil {
		return err
	}
	csvTempPath := csvTemp.Name()
	defer os.Remove(csvTempPath)
	if err := csvTemp.Close(); err != nil {
		return err
	}

	mmdbTemp, err := os.CreateTemp(config.dataDir, ".ipinfo-lite-mmdb-*.mmdb")
	if err != nil {
		return err
	}
	mmdbTempPath := mmdbTemp.Name()
	defer os.Remove(mmdbTempPath)
	if err := mmdbTemp.Close(); err != nil {
		return err
	}

	fmt.Println("Downloading official IPinfo Lite CSV...")
	if err := downloadVerified(config.client, config.csvURL, config.token, csvGzipPath); err != nil {
		return fmt.Errorf("download CSV: %w", err)
	}
	if err := gunzipFile(csvGzipPath, csvTempPath); err != nil {
		return fmt.Errorf("extract CSV: %w", err)
	}

	fmt.Println("Downloading official IPinfo Lite MMDB...")
	if err := downloadVerified(config.client, config.mmdbURL, config.token, mmdbTempPath); err != nil {
		return fmt.Errorf("download MMDB: %w", err)
	}
	if err := os.Chmod(csvTempPath, 0o644); err != nil {
		return fmt.Errorf("set CSV permissions: %w", err)
	}
	if err := os.Chmod(mmdbTempPath, 0o644); err != nil {
		return fmt.Errorf("set MMDB permissions: %w", err)
	}

	if err := os.Rename(mmdbTempPath, mmdbPath); err != nil {
		return fmt.Errorf("install MMDB: %w", err)
	}
	if err := os.Rename(csvTempPath, csvPath); err != nil {
		return fmt.Errorf("install CSV: %w", err)
	}
	if err := writeFileAtomic(versionPath, []byte(config.today+"\n"), 0o644); err != nil {
		return fmt.Errorf("write version: %w", err)
	}

	fmt.Println("Official IPinfo Lite CSV and MMDB updated successfully")
	return nil
}

func downloadVerified(client *http.Client, rawURL, token, destination string) error {
	expected, err := fetchSHA256(client, rawURL+"/checksums", token)
	if err != nil {
		return fmt.Errorf("fetch checksum: %w", err)
	}
	response, err := getIPinfo(client, rawURL, token)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(output, hash), response.Body)
	closeErr := output.Close()
	if copyErr != nil {
		return fmt.Errorf("read response: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

func fetchSHA256(client *http.Client, rawURL, token string) (string, error) {
	response, err := getIPinfo(client, rawURL, token)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var payload struct {
		Checksums struct {
			SHA256 string `json:"sha256"`
		} `json:"checksums"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode checksum: %w", err)
	}
	digest := strings.ToLower(payload.Checksums.SHA256)
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("invalid SHA-256 checksum %q", payload.Checksums.SHA256)
	}
	return digest, nil
}

func getIPinfo(client *http.Client, rawURL, token string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("token", token)
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "ipinfo-csv-lite/"+version)
	response, err := client.Do(request)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, fmt.Errorf("request failed: %w", urlErr.Err)
		}
		return nil, errors.New("request failed")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("request failed: %s", response.Status)
	}
	return response, nil
}

func currentVersion(path string) string {
	stored, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(strings.TrimSpace(string(stored)))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
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

	fOut, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fOut, zr); err != nil {
		fOut.Close()
		return err
	}
	return fOut.Close()
}
