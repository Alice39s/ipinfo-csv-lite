package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// runRelease compresses data/ipinfo-lite.csv to gzip, xz and zstd formats,
// all written concurrently.
func runRelease() error {
	input := filepath.Join(dataDir, "ipinfo-lite.csv")
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("%s not found", input)
	}

	targets := []struct {
		ext       string
		newWriter func(io.Writer) (io.WriteCloser, error)
	}{
		{".gz", func(w io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(w), nil }},
		{".xz", func(w io.Writer) (io.WriteCloser, error) { return xz.NewWriter(w) }},
		{".zst", func(w io.Writer) (io.WriteCloser, error) { return zstd.NewWriter(w) }},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(targets))

	for _, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dst := input + t.ext
			fmt.Printf("Creating %s...\n", dst)
			if err := compressFile(input, dst, t.newWriter); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}

	fmt.Println("Compression completed.")
	return nil
}

func compressFile(src, dst string, newWriter func(io.Writer) (io.WriteCloser, error)) error {
	fIn, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fIn.Close()

	fOut, err := os.Create(dst)
	if err != nil {
		return err
	}

	w, err := newWriter(fOut)
	if err != nil {
		fOut.Close()
		return err
	}

	if _, err := io.Copy(w, fIn); err != nil {
		w.Close()
		fOut.Close()
		return err
	}
	if err := w.Close(); err != nil {
		fOut.Close()
		return err
	}
	return fOut.Close()
}
