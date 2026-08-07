package main

import (
	"bufio"
	"encoding/csv"
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

	fIn, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("open %s: %w", input, err)
	}
	defer fIn.Close()

	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "ipinfo-lite",
		Description:             map[string]string{"en": "IPinfo lite geolocation database (country + continent + ASN)"},
		Languages:               []string{"en"},
		RecordSize:              28,
		IPVersion:               6, // IPv6 tree holds both IPv4 and IPv6 networks
		IncludeReservedNetworks: true,
	})
	if err != nil {
		return err
	}

	reader := csv.NewReader(bufio.NewReaderSize(fIn, 1<<20))
	reader.FieldsPerRecord = -1
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	var inserted, skipped int
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		record, ok := mmdbRecord(row)
		if !ok {
			skipped++
			continue
		}
		network, err := parseNetwork(row[0])
		if err != nil {
			skipped++
			continue
		}
		if err := tree.Insert(network, record); err != nil {
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

// mmdbRecord maps a lite CSV row (cidr, country_code, continent_code,
// as_number, as_name) to an MMDB record. ok is false for malformed rows.
func mmdbRecord(row []string) (mmdbtype.Map, bool) {
	if len(row) < 5 || row[0] == "" {
		return nil, false
	}

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
	return record, true
}
