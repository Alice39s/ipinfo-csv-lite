package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

// runMmdb converts data/ipinfo-lite.csv into a MaxMind DB file at
// data/ipinfo-lite.mmdb. The record structure is our own:
//
//	{
//	  "country_code":   "US",        // omitted when empty
//	  "continent_code": "NA",        // omitted when empty
//	  "as_number":      15169,       // uint32, always present
//	  "as_name":        "Google LLC" // omitted when empty
//	}
func runMmdb() error {
	input := filepath.Join(dataDir, "ipinfo-lite.csv")
	output := filepath.Join(dataDir, "ipinfo-lite.mmdb")

	fIn, reader, err := openLiteCSV(input)
	if err != nil {
		return err
	}
	defer fIn.Close()

	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "ipinfo-lite",
		Description:             map[string]string{"en": "IPinfo lite geolocation database (country + continent + ASN)"},
		Languages:               []string{"en"},
		RecordSize:              28,
		IPVersion:               6, // IPv6 tree holds both IPv4 and IPv6 networks
		IncludeReservedNetworks: true,
		KeyGenerator:            keyGenFunc(recordKey),
	})
	if err != nil {
		return err
	}

	var inserted, skipped int
	// The 3.47M source rows collapse to only ~120k unique records; build each
	// mmdbtype.Map once and reuse it. mmdbwriter's dataMap already dedups the
	// serialized record, so reusing one Map object across many inserts is safe
	// (the default inserter never mutates the value it receives) and removes
	// ~1.2 GB of per-row Map allocations plus the associated GC churn.
	records := make(map[string]mmdbtype.Map, 1<<17)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		key, ok := recordDedupKey(row)
		if !ok {
			skipped++
			continue
		}
		// Reuse the Map for every network that shares this record tuple;
		// only build it on the first sighting.
		rec, hit := records[key]
		if !hit {
			rec = buildMmdbRecord(row)
			records[key] = rec
		}
		network, err := parseNetwork(row[0])
		if err != nil {
			skipped++
			continue
		}
		if err := tree.Insert(network, rec); err != nil {
			return fmt.Errorf("insert %s: %w", row[0], err)
		}
		inserted++
	}

	fOut, err := os.Create(output)
	if err != nil {
		return err
	}
	if _, err := tree.WriteTo(fOut); err != nil {
		fOut.Close()
		return fmt.Errorf("write %s: %w", output, err)
	}
	if err := fOut.Close(); err != nil {
		return err
	}

	fmt.Printf("Wrote %s (%d networks, %d skipped)\n", output, inserted, skipped)
	return nil
}

// keyGenFunc adapts a function to the mmdbwriter.KeyGenerator interface.
type keyGenFunc func(mmdbtype.DataType) ([]byte, error)

func (f keyGenFunc) Key(v mmdbtype.DataType) ([]byte, error) { return f(v) }

// recordKey generates the dedup key for our flat record structure directly,
// avoiding the default serializer+SHA-256 key generator which is slow for
// millions of inserts. Fields are joined with NUL separators (never present
// in CSV values), so distinct records always produce distinct keys.
func recordKey(v mmdbtype.DataType) ([]byte, error) {
	m, ok := v.(mmdbtype.Map)
	if !ok {
		return nil, fmt.Errorf("unexpected record type %T", v)
	}
	key := make([]byte, 0, 64)
	for _, field := range []string{"country_code", "continent_code", "as_number", "as_name"} {
		switch value := m[mmdbtype.String(field)].(type) {
		case mmdbtype.String:
			key = append(key, value...)
		case mmdbtype.Uint32:
			key = strconv.AppendUint(key, uint64(value), 10)
		}
		key = append(key, 0)
	}
	return key, nil
}

// parseNetwork parses a CIDR, tolerating plain IP addresses (treated as
// /32 or /128) which occur in the source data.
func parseNetwork(cidr string) (*net.IPNet, error) {
	if _, network, err := net.ParseCIDR(cidr); err == nil {
		return network, nil
	}
	ip := net.ParseIP(cidr)
	if ip == nil {
		return nil, fmt.Errorf("invalid network %q", cidr)
	}
	bits := 128
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		bits = 32
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, nil
}

// recordDedupKey validates a lite CSV row (cidr, country_code, continent_code,
// as_number, as_name) and returns a stable key over its four output fields.
// ok is false for malformed rows (too few columns or empty network). The key
// lets runMmdb build one mmdbtype.Map per unique record instead of per row.
// Fields are NUL-separated; NUL never appears in CSV cell values.
func recordDedupKey(row []string) (string, bool) {
	if len(row) < 5 || row[0] == "" {
		return "", false
	}
	return row[1] + "\x00" + row[2] + "\x00" + row[3] + "\x00" + row[4], true
}

// buildMmdbRecord constructs the mmdbtype.Map for a validated row. Empty
// optional fields are omitted; as_number is always present (uint32).
func buildMmdbRecord(row []string) mmdbtype.Map {
	asNumber, err := strconv.ParseUint(row[3], 10, 32)
	if err != nil {
		asNumber = 0
	}
	record := mmdbtype.Map{
		"as_number": mmdbtype.Uint32(asNumber),
	}
	if row[1] != "" {
		record["country_code"] = mmdbtype.String(row[1])
	}
	if row[2] != "" {
		record["continent_code"] = mmdbtype.String(row[2])
	}
	if row[4] != "" {
		record["as_name"] = mmdbtype.String(row[4])
	}
	return record
}

// mmdbRecord is a convenience wrapper combining recordDedupKey and
// buildMmdbRecord, used by tests. Production code (runMmdb) splits them so it
// can skip the Map allocation on cache hits.
func mmdbRecord(row []string) (record mmdbtype.Map, key string, ok bool) {
	key, ok = recordDedupKey(row)
	if !ok {
		return nil, "", false
	}
	return buildMmdbRecord(row), key, true
}
