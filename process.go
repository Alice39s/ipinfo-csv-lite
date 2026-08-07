package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Input columns (IPinfo "lite" format):
//
//	0: network (CIDR), 1: country, 2: country_code, 3: continent,
//	4: continent_code, 5: asn, 6: as_name, 7: as_domain
//
// Output columns: cidr, country_code, continent_code, as_number, as_name
func runProcess() error {
	input := filepath.Join(dataDir, "country_asn.csv")
	output := filepath.Join(dataDir, "ipinfo-lite.csv")

	fIn, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("open %s: %w", input, err)
	}
	defer fIn.Close()

	fOut, err := os.Create(output)
	if err != nil {
		return err
	}

	reader := csv.NewReader(fIn)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; short ones are skipped below
	if _, err := reader.Read(); err != nil {
		fOut.Close()
		return fmt.Errorf("read header: %w", err)
	}

	writer := csv.NewWriter(fOut)
	if err := writer.Write([]string{"cidr", "country_code", "continent_code", "as_number", "as_name"}); err != nil {
		fOut.Close()
		return err
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fOut.Close()
			return err
		}

		// Skip malformed rows
		if len(row) < 7 {
			continue
		}
		network := strings.TrimSpace(row[0])
		if network == "" {
			continue
		}

		if err := writer.Write([]string{
			network,                        // cidr (already in CIDR format)
			row[2],                         // country_code
			row[4],                         // continent_code
			strconv.Itoa(parseASN(row[5])), // as_number
			row[6],                         // as_name (may be empty)
		}); err != nil {
			fOut.Close()
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		fOut.Close()
		return err
	}
	if err := fOut.Close(); err != nil {
		return err
	}

	fmt.Printf("Processed %s -> %s\n", input, output)
	return nil
}

// parseASN strips the "AS" prefix; empty or unparsable values become 0.
func parseASN(asn string) int {
	n, err := strconv.Atoi(strings.TrimSpace(strings.ReplaceAll(asn, "AS", "")))
	if err != nil {
		return 0
	}
	return n
}
