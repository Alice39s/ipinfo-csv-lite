package main

import (
	"fmt"
	"os"
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
	for _, step := range []func() error{runUpdate, runProcess, runRelease, runMmdb, runXdb, runChecksum} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
