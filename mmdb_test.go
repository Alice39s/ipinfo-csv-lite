package main

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func TestMmdbRecord(t *testing.T) {
	record, ok := mmdbRecord([]string{"1.0.0.0/24", "AU", "OC", "13335", "Cloudflare"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if _, ok := record["country_code"]; !ok {
		t.Error("missing country_code")
	}
	if _, ok := record["as_name"]; !ok {
		t.Error("missing as_name")
	}

	// Empty fields are omitted; as_number is always present.
	record, ok = mmdbRecord([]string{"57.144.0.0/16", "", "", "0", ""})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if _, ok := record["country_code"]; ok {
		t.Error("country_code should be omitted when empty")
	}
	if _, ok := record["as_name"]; ok {
		t.Error("as_name should be omitted when empty")
	}
	if _, ok := record["as_number"]; !ok {
		t.Error("as_number should always be present")
	}

	if _, ok := mmdbRecord([]string{"", "AU", "OC", "0", ""}); ok {
		t.Error("expected ok=false for empty cidr")
	}
	if _, ok := mmdbRecord([]string{"short"}); ok {
		t.Error("expected ok=false for short row")
	}
}

func TestMmdbSmoke(t *testing.T) {
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "ipinfo-lite",
		Description:             map[string]string{"en": "test"},
		Languages:               []string{"en"},
		RecordSize:              28,
		IPVersion:               6,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatalf("mmdbwriter.New: %v", err)
	}

	rows := [][]string{
		{"1.0.0.0/24", "AU", "OC", "13335", "Cloudflare"},
		{"2001:4860:4860::/48", "US", "NA", "15169", "Google LLC"},
	}
	for _, row := range rows {
		record, ok := mmdbRecord(row)
		if !ok {
			t.Fatalf("mmdbRecord(%v) not ok", row)
		}
		_, network, err := net.ParseCIDR(row[0])
		if err != nil {
			t.Fatalf("ParseCIDR(%s): %v", row[0], err)
		}
		if err := tree.Insert(network, record); err != nil {
			t.Fatalf("insert %s: %v", row[0], err)
		}
	}

	path := filepath.Join(t.TempDir(), "test.mmdb")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := tree.WriteTo(f); err != nil {
		f.Close()
		t.Fatalf("WriteTo: %v", err)
	}
	f.Close()

	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatalf("open mmdb: %v", err)
	}
	defer db.Close()

	if db.Metadata.DatabaseType != "ipinfo-lite" {
		t.Errorf("database type = %q, want ipinfo-lite", db.Metadata.DatabaseType)
	}

	var record struct {
		CountryCode   string `maxminddb:"country_code"`
		ContinentCode string `maxminddb:"continent_code"`
		ASNumber      uint32 `maxminddb:"as_number"`
		ASName        string `maxminddb:"as_name"`
	}

	result := db.Lookup(netip.MustParseAddr("1.0.0.1"))
	if !result.Found() {
		t.Fatal("1.0.0.1 not found")
	}
	if err := result.Decode(&record); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if record.CountryCode != "AU" || record.ContinentCode != "OC" || record.ASNumber != 13335 || record.ASName != "Cloudflare" {
		t.Errorf("v4 record = %+v", record)
	}

	result = db.Lookup(netip.MustParseAddr("2001:4860:4860::8888"))
	if !result.Found() {
		t.Fatal("2001:4860:4860::8888 not found")
	}
	if err := result.Decode(&record); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if record.CountryCode != "US" || record.ASNumber != 15169 || record.ASName != "Google LLC" {
		t.Errorf("v6 record = %+v", record)
	}

	if db.Lookup(netip.MustParseAddr("2.2.2.2")).Found() {
		t.Error("2.2.2.2 should not be found")
	}
}
