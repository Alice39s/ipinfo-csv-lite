package main

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func TestMmdbRecordFromRow(t *testing.T) {
	record := mmdbRecordFromRow([]string{"1.0.0.0/24", "AU", "OC", "13335", "Cloudflare"})
	if record.countryCode != "AU" || record.continentCode != "OC" || record.asNumber != 13335 || record.asName != "Cloudflare" {
		t.Fatalf("record = %+v", record)
	}
	record = mmdbRecordFromRow([]string{"1.0.0.0/24", "", "", "invalid", ""})
	if record.asNumber != 0 {
		t.Fatalf("invalid ASN = %d, want 0", record.asNumber)
	}
}

func TestParseMmdbPrefix(t *testing.T) {
	prefix, err := parseLitePrefix("1.7.168.174")
	if err != nil {
		t.Fatalf("plain IPv4: %v", err)
	}
	if prefix.Bits() != 32 {
		t.Errorf("plain IPv4 prefix = /%d, want /32", prefix.Bits())
	}

	prefix, err = parseLitePrefix("2001:4860:4860::8888")
	if err != nil {
		t.Fatalf("plain IPv6: %v", err)
	}
	if prefix.Bits() != 128 {
		t.Errorf("plain IPv6 prefix = /%d, want /128", prefix.Bits())
	}

	if _, err = parseLitePrefix("not-an-ip"); err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestMmdbSmoke(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.csv")
	output := filepath.Join(dir, "test.mmdb")
	csv := "cidr,country_code,continent_code,as_number,as_name\n" +
		"1.0.0.0/24,AU,OC,13335,Cloudflare\n" +
		"1.0.0.128/25,US,NA,15169,Google\n" +
		"2001:4860:4860::/48,US,NA,15169,Google\n" +
		"57.144.0.0/16,,,0,\n" +
		"invalid,US,NA,0,Invalid\n"
	if err := os.WriteFile(input, []byte(csv), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	inserted, skipped, err := writeMmdb(input, output, 1_700_000_000)
	if err != nil {
		t.Fatalf("writeMmdb: %v", err)
	}
	if inserted != 4 || skipped != 1 {
		t.Fatalf("inserted/skipped = %d/%d, want 4/1", inserted, skipped)
	}

	db, err := maxminddb.Open(output)
	if err != nil {
		t.Fatalf("open mmdb: %v", err)
	}
	defer db.Close()

	if db.Metadata.DatabaseType != "ipinfo-lite" || db.Metadata.IPVersion != 6 || db.Metadata.RecordSize != 28 {
		t.Errorf("metadata = %+v", db.Metadata)
	}

	tests := []struct {
		ip      string
		found   bool
		country string
		asn     uint32
		name    string
	}{
		{"1.0.0.1", true, "AU", 13335, "Cloudflare"},
		{"1.0.0.200", true, "US", 15169, "Google"},
		{"::ffff:1.0.0.1", true, "AU", 13335, "Cloudflare"},
		{"2002:0100:0001::", true, "AU", 13335, "Cloudflare"},
		{"2001:4860:4860::8888", true, "US", 15169, "Google"},
		{"57.144.0.1", true, "", 0, ""},
		{"2.2.2.2", false, "", 0, ""},
	}
	for _, test := range tests {
		var record struct {
			CountryCode   string `maxminddb:"country_code"`
			ContinentCode string `maxminddb:"continent_code"`
			ASNumber      uint32 `maxminddb:"as_number"`
			ASName        string `maxminddb:"as_name"`
		}
		result := db.Lookup(netip.MustParseAddr(test.ip))
		if result.Found() != test.found {
			t.Errorf("lookup %s found = %v, want %v", test.ip, result.Found(), test.found)
			continue
		}
		if !test.found {
			continue
		}
		if err := result.Decode(&record); err != nil {
			t.Errorf("decode %s: %v", test.ip, err)
			continue
		}
		if record.CountryCode != test.country || record.ASNumber != test.asn || record.ASName != test.name {
			t.Errorf("lookup %s = %+v", test.ip, record)
		}
	}
}
