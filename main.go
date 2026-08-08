package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

// dataDir holds the downloaded source CSV and the generated artifacts.
const dataDir = "data"

func usage() {
	fmt.Fprintf(os.Stderr, `ipinfo-lite %s — lightweight IPinfo CSV database pipeline

Usage:
  ipinfo-lite <command>

Commands:
  update    Download and extract the latest IPinfo database (requires IPINFO_TOKEN)
  process   Reduce the raw CSV to the lite schema
  release   Compress the output to .gz, .xz and .zst
  mmdb      Convert the output to MaxMind DB format (.mmdb)
  xdb       Convert the output to ip2region xdb format (.ipv4.xdb / .ipv6.xdb)
  checksum  Write checksums.txt with SHA-256 of all release artifacts
  generate  Build all artifacts from existing source data (no download)
  all       Run all of the above in sequence
`, version)
}

func main() {
	if err := loadDotenv(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load .env: %v\n", err)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "update":
		err = runUpdate()
	case "process":
		err = runProcess()
	case "release":
		err = runRelease()
	case "mmdb":
		err = runMmdb()
	case "xdb":
		err = runXdb()
	case "checksum":
		err = runChecksum()
	case "generate":
		err = runGenerate()
	case "all":
		err = runAll()
	case "-h", "--help", "help":
		usage()
		return
	case "-v", "--version", "version":
		fmt.Println(version)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runAll() error {
	if err := runUpdate(); err != nil {
		return err
	}
	return runGenerate()
}

func runGenerate() error {
	if err := runArtifactPipeline(); err != nil {
		return err
	}
	return runChecksum()
}

// runArtifactPipeline shares process's ordered output batches with the MMDB and
// XDB builders. This removes two full CSV parse passes; compression starts as
// soon as the final CSV is closed while both database writers finish.
func runArtifactPipeline() error {
	bufferSize := max(2, runtime.NumCPU())
	mmdbRows := make(chan [][]string, bufferSize)
	xdbRows := make(chan [][]string, bufferSize)
	errs := make(chan error, 3)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		output := filepath.Join(dataDir, "ipinfo-lite.mmdb")
		inserted, skipped, err := writeMmdbBatches(mmdbRows, output, time.Now().Unix())
		if err != nil {
			errs <- err
			return
		}
		fmt.Printf("Wrote %s (%d networks, %d skipped)\n", output, inserted, skipped)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := writeXdbBatches(xdbRows); err != nil {
			errs <- err
		}
	}()

	processErr := runProcessWithConsumer(func(rows [][]string) error {
		mmdbRows <- rows
		xdbRows <- rows
		return nil
	})
	close(mmdbRows)
	close(xdbRows)

	if processErr == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runRelease(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	allErrors := []error{processErr}
	for err := range errs {
		allErrors = append(allErrors, err)
	}
	return errors.Join(allErrors...)
}
