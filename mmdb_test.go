package main

import (
	"bytes"
	"encoding/base64"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func TestPublishOfficialMmdbPreservesBytesAndSchema(t *testing.T) {
	sourceBytes := officialSampleMmdb(t)
	dir := t.TempDir()
	source := filepath.Join(dir, sourceMMDBName)
	destination := filepath.Join(dir, "ipinfo-lite.mmdb")
	if err := os.WriteFile(source, sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := publishOfficialMmdb(source, destination); err != nil {
		t.Fatalf("publishOfficialMmdb: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sourceBytes) {
		t.Fatal("published MMDB differs from the official source bytes")
	}
	assertFileMode(t, destination, 0o644)

	database, err := maxminddb.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var record struct {
		ASN           string `maxminddb:"asn"`
		ASDomain      string `maxminddb:"as_domain"`
		ASName        string `maxminddb:"as_name"`
		Continent     string `maxminddb:"continent"`
		ContinentCode string `maxminddb:"continent_code"`
		Country       string `maxminddb:"country"`
		CountryCode   string `maxminddb:"country_code"`
	}
	result := database.Lookup(netip.MustParseAddr("1.0.0.1"))
	if !result.Found() {
		t.Fatal("official sample has no record for 1.0.0.1")
	}
	if err := result.Decode(&record); err != nil {
		t.Fatal(err)
	}
	if record.ASN != "AS13335" || record.ASDomain != "cloudflare.com" ||
		record.ASName != "Cloudflare, Inc." || record.Country != "Australia" ||
		record.CountryCode != "AU" || record.Continent != "Oceania" || record.ContinentCode != "OC" {
		t.Fatalf("unexpected official record: %+v", record)
	}
}

func TestPublishOfficialMmdbRejectsInvalidSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, sourceMMDBName)
	destination := filepath.Join(dir, "ipinfo-lite.mmdb")
	old := []byte("existing release")
	if err := os.WriteFile(source, []byte("not an MMDB"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, old, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := publishOfficialMmdb(source, destination); err == nil {
		t.Fatal("publishOfficialMmdb accepted an invalid MMDB")
	}
	assertFileContent(t, destination, old)
}

// officialSampleMmdb loads the public fixture from
// https://ipinfo.io/data/sample/ipinfo_lite.mmdb (SHA-256
// 4df37d4daf55e11b22176ca40ed6e124e8435efaa25d5018f2ac9d6185576e1c).
func officialSampleMmdb(t *testing.T) []byte {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join("testdata", "ipinfo_lite_sample.mmdb.b64"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
