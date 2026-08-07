package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// processChunkSize is the number of CSV rows handed to each worker per job.
const processChunkSize = 20000

// Input columns (IPinfo "lite" format):
//
//	0: network (CIDR), 1: country, 2: country_code, 3: continent,
//	4: continent_code, 5: asn, 6: as_name, 7: as_domain
//
// Output columns: cidr, country_code, continent_code, as_number, as_name
func runProcess() error {
	input := filepath.Join(dataDir, "country_asn.csv")
	output := filepath.Join(dataDir, "ipinfo-lite.csv")

	fIn, reader, err := openLiteCSV(input)
	if err != nil {
		return err
	}
	defer fIn.Close()

	fOut, err := os.Create(output)
	if err != nil {
		return err
	}

	writer := csv.NewWriter(bufio.NewWriterSize(fOut, 1<<20))
	if err := writer.Write([]string{"cidr", "country_code", "continent_code", "as_number", "as_name"}); err != nil {
		fOut.Close()
		return err
	}

	if err := transformCSV(reader, writer, runtime.NumCPU()); err != nil {
		fOut.Close()
		return err
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

// transformCSV reads rows from reader, transforms them in parallel workers
// and writes the results to writer in the original row order.
func transformCSV(reader *csv.Reader, writer *csv.Writer, workers int) error {
	if workers < 1 {
		workers = 1
	}

	type job struct {
		idx  int
		rows [][]string
	}
	type result struct {
		idx  int
		rows [][]string
	}

	jobs := make(chan job, workers)
	results := make(chan result, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				out := make([][]string, 0, len(j.rows))
				for _, row := range j.rows {
					if r, ok := transformRow(row); ok {
						out = append(out, r)
					}
				}
				results <- result{idx: j.idx, rows: out}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	// The collector writes results in job order. It runs concurrently with
	// the dispatch loop below: collecting only after all jobs are dispatched
	// would deadlock once the results channel fills up.
	var writeErr error
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		pending := make(map[int][][]string)
		nextWrite := 0
		for r := range results {
			pending[r.idx] = r.rows
			for {
				rows, ok := pending[nextWrite]
				if !ok {
					break
				}
				if writeErr == nil {
					if err := writer.WriteAll(rows); err != nil {
						writeErr = err
					}
				}
				delete(pending, nextWrite)
				nextWrite++
			}
		}
	}()

	// Read chunks and dispatch jobs until EOF or a read error.
	var readErr error
	nextJob := 0
	for {
		chunk := make([][]string, 0, processChunkSize)
		for len(chunk) < processChunkSize {
			row, err := reader.Read()
			if err != nil {
				if err != io.EOF {
					readErr = err
				}
				break
			}
			chunk = append(chunk, row)
		}
		if len(chunk) == 0 {
			break
		}
		jobs <- job{idx: nextJob, rows: chunk}
		nextJob++
		if readErr != nil {
			break
		}
	}
	close(jobs)

	<-collectorDone
	if writeErr != nil {
		return writeErr
	}
	return readErr
}

// transformRow maps a raw IPinfo lite row to the lite schema.
//
// `ok` is false for rows that should be skipped (malformed or empty network).
func transformRow(row []string) (out []string, ok bool) {
	if len(row) < 7 {
		return nil, false
	}
	network := strings.TrimSpace(row[0])
	if network == "" {
		return nil, false
	}
	return []string{
		network,                        // cidr (already in CIDR format)
		row[2],                         // country_code
		row[4],                         // continent_code
		strconv.Itoa(parseASN(row[5])), // as_number
		row[6],                         // as_name (may be empty)
	}, true
}

// parseASN strips a leading "AS" prefix; empty or unparsable values become 0.
// Only a single leading prefix is removed (not every "AS" substring), so an
// AS name containing "AS" internally would not corrupt the number.
func parseASN(asn string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(asn), "AS"))
	if err != nil {
		return 0
	}
	return n
}
