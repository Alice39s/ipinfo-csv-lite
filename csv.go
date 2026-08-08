package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net/netip"
	"os"
)

// liteCSVReadSize is the buffered-I/O size used when reading the source CSVs.
const liteCSVReadSize = 1 << 20

// parseLitePrefix accepts both CIDRs and plain addresses from the source.
func parseLitePrefix(value string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(value)
	if err == nil {
		return prefix.Masked(), nil
	}
	addr, addrErr := netip.ParseAddr(value)
	if addrErr != nil {
		return netip.Prefix{}, fmt.Errorf("invalid network %q", value)
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// openLiteCSV opens a CSV file, wraps it in a buffered reader tolerating ragged
// rows (FieldsPerRecord = -1) and consumes the header line. The returned file
// must be closed by the caller, e.g. via `defer f.Close()`.
//
// `runProcess` reads the raw country_asn.csv and `runMmdb`/`runXdb` read the
// reduced ipinfo-lite.csv; both share this exact setup, so it lives here.
func openLiteCSV(path string, reuseRecord bool) (*os.File, *csv.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	reader := csv.NewReader(bufio.NewReaderSize(f, liteCSVReadSize))
	reader.FieldsPerRecord = -1 // tolerate ragged rows; short ones are skipped by callers
	reader.ReuseRecord = reuseRecord
	if _, err := reader.Read(); err != nil { // skip header
		f.Close()
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	return f, reader, nil
}
