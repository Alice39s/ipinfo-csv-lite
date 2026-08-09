package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestUpdateDatabaseDownloadsOfficialFiles(t *testing.T) {
	csv := []byte("network,country,country_code,continent,continent_code,asn,as_name,as_domain\n1.0.0.0/24,Australia,AU,Oceania,OC,AS13335,Cloudflare,cloudflare.com\n")
	csvGzip := gzipBytes(t, csv)
	mmdb := []byte("official-mmdb")
	server, requests := newIPinfoDownloadServer(t, csvGzip, mmdb, false)
	defer server.Close()

	dir := t.TempDir()
	err := updateDatabase(updateConfig{
		client:  server.Client(),
		dataDir: dir,
		token:   "test-token",
		today:   "2026-08-08",
		csvURL:  server.URL + "/ipinfo_lite.csv.gz",
		mmdbURL: server.URL + "/ipinfo_lite.mmdb",
	})
	if err != nil {
		t.Fatalf("updateDatabase: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, sourceCSVName), csv)
	assertFileContent(t, filepath.Join(dir, sourceMMDBName), mmdb)
	assertFileContent(t, filepath.Join(dir, versionFileName), []byte("2026-08-08\n"))
	assertFileMode(t, filepath.Join(dir, sourceCSVName), 0o644)
	assertFileMode(t, filepath.Join(dir, sourceMMDBName), 0o644)

	requests.Lock()
	if len(requests.paths) != 4 {
		t.Errorf("request count = %d, want 4: %v", len(requests.paths), requests.paths)
	}
	requests.Unlock()

	if err := updateDatabase(updateConfig{
		client:  server.Client(),
		dataDir: dir,
		token:   "test-token",
		today:   "2026-08-08",
		csvURL:  server.URL + "/ipinfo_lite.csv.gz",
		mmdbURL: server.URL + "/ipinfo_lite.mmdb",
	}); err != nil {
		t.Fatalf("cached updateDatabase: %v", err)
	}
	requests.Lock()
	if len(requests.paths) != 4 {
		t.Errorf("cached request count = %d, want 4", len(requests.paths))
	}
	requests.Unlock()

	if err := os.Truncate(filepath.Join(dir, sourceMMDBName), 0); err != nil {
		t.Fatal(err)
	}
	if err := updateDatabase(updateConfig{
		client:  server.Client(),
		dataDir: dir,
		token:   "test-token",
		today:   "2026-08-08",
		csvURL:  server.URL + "/ipinfo_lite.csv.gz",
		mmdbURL: server.URL + "/ipinfo_lite.mmdb",
	}); err != nil {
		t.Fatalf("repair empty cache: %v", err)
	}
	assertFileContent(t, filepath.Join(dir, sourceMMDBName), mmdb)
	requests.Lock()
	defer requests.Unlock()
	if len(requests.paths) != 8 {
		t.Errorf("repair request count = %d, want 8", len(requests.paths))
	}
}

func TestUpdateDatabaseChecksumFailurePreservesExistingFiles(t *testing.T) {
	csvGzip := gzipBytes(t, []byte("new csv"))
	server, _ := newIPinfoDownloadServer(t, csvGzip, []byte("new mmdb"), true)
	defer server.Close()

	dir := t.TempDir()
	oldCSV := []byte("old csv")
	oldMMDB := []byte("old mmdb")
	if err := os.WriteFile(filepath.Join(dir, sourceCSVName), oldCSV, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sourceMMDBName), oldMMDB, 0o600); err != nil {
		t.Fatal(err)
	}

	err := updateDatabase(updateConfig{
		client:  server.Client(),
		dataDir: dir,
		token:   "test-token",
		today:   "2026-08-08",
		csvURL:  server.URL + "/ipinfo_lite.csv.gz",
		mmdbURL: server.URL + "/ipinfo_lite.mmdb",
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
	assertFileContent(t, filepath.Join(dir, sourceCSVName), oldCSV)
	assertFileContent(t, filepath.Join(dir, sourceMMDBName), oldMMDB)
	if _, err := os.Stat(filepath.Join(dir, versionFileName)); !os.IsNotExist(err) {
		t.Fatalf("version file error = %v, want not exist", err)
	}
}

func TestGetIPinfoDoesNotLeakTokenOnTransportError(t *testing.T) {
	const token = "super-secret-token"
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport exploded")
	})}

	_, err := getIPinfo(client, "https://example.invalid/data", token)
	if err == nil {
		t.Fatal("getIPinfo returned no error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaks token: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type requestLog struct {
	sync.Mutex
	paths []string
}

func newIPinfoDownloadServer(t *testing.T, csvGzip, mmdb []byte, badMMDBChecksum bool) (*httptest.Server, *requestLog) {
	t.Helper()
	requests := &requestLog{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "test-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		requests.Lock()
		requests.paths = append(requests.paths, r.URL.Path)
		requests.Unlock()

		var payload []byte
		switch strings.TrimSuffix(r.URL.Path, "/checksums") {
		case "/ipinfo_lite.csv.gz":
			payload = csvGzip
		case "/ipinfo_lite.mmdb":
			payload = mmdb
		default:
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/checksums") {
			sum := sha256.Sum256(payload)
			digest := hex.EncodeToString(sum[:])
			if badMMDBChecksum && r.URL.Path == "/ipinfo_lite.mmdb/checksums" {
				digest = strings.Repeat("0", sha256.Size*2)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"checksums":{"sha256":%q}}`, digest)
			return
		}
		_, _ = w.Write(payload)
	}))
	return server, requests
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	w := gzip.NewWriter(&output)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %o, want %o", path, got, want)
	}
}
