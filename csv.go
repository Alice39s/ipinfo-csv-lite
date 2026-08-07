package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
)

// liteCSVReadSize is the buffered-I/O size used when reading the source CSVs.
const liteCSVReadSize = 1 << 20

// openLiteCSV opens a CSV file, wraps it in a buffered reader tolerating ragged
// rows (FieldsPerRecord = -1) and consumes the header line. The returned file
// must be closed by the caller, e.g. via `defer f.Close()`.
//
// `runProcess` reads the raw country_asn.csv and `runMmdb`/`runXdb` read the
// reduced ipinfo-lite.csv; both share this exact setup, so it lives here.
func openLiteCSV(path string) (*os.File, *csv.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	reader := csv.NewReader(bufio.NewReaderSize(f, liteCSVReadSize))
	reader.FieldsPerRecord = -1              // tolerate ragged rows; short ones are skipped by callers
	if _, err := reader.Read(); err != nil { // skip header
		f.Close()
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	return f, reader, nil
}
