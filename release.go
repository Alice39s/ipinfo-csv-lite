package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

const (
	xzStreams        = 4
	xzDictionarySize = 1 << 14
)

// runRelease compresses data/ipinfo-lite.csv to gzip, xz and zstd formats,
// all written concurrently.
func runRelease() error {
	input := filepath.Join(dataDir, "ipinfo-lite.csv")
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("%s not found", input)
	}

	targets := []struct {
		ext      string
		compress func(src, dst string) error
	}{
		{".gz", func(src, dst string) error {
			// BestSpeed is over three times faster on the CI-class benchmark host;
			// the roughly 4 MiB size trade-off is modest for this daily artifact.
			return compressFile(src, dst, func(w io.Writer) (io.WriteCloser, error) {
				return gzip.NewWriterLevel(w, gzip.BestSpeed)
			})
		}},
		{".xz", func(src, dst string) error {
			// Prefer liblzma's parallel CLI when installed (including GitHub's
			// Ubuntu runners), retaining the pure-Go path as a portable fallback.
			return compressXZFile(src, dst, xzStreams)
		}},
		{".zst", func(src, dst string) error {
			return compressFile(src, dst, func(w io.Writer) (io.WriteCloser, error) {
				return zstd.NewWriter(w,
					zstd.WithEncoderLevel(zstd.SpeedFastest),
					zstd.WithEncoderConcurrency(2),
				)
			})
		}},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(targets))

	for _, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dst := input + t.ext
			fmt.Printf("Creating %s...\n", dst)
			if err := t.compress(input, dst); err != nil {
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

// compressXZFile uses the optimized liblzma CLI when available and otherwise
// falls back to deterministic concatenated streams from the pure-Go encoder.
func compressXZFile(src, dst string, streams int) error {
	if streams < 1 {
		return fmt.Errorf("xz streams must be positive")
	}
	if command, err := exec.LookPath("xz"); err == nil {
		return compressXZCommand(command, src, dst, streams)
	}
	config := xz.WriterConfig{
		DictCap:    xzDictionarySize,
		Properties: &lzma.Properties{LC: 2, PB: 0},
	}
	return compressXZFileWithConfig(src, dst, streams, config)
}

func compressXZCommand(command, src, dst string, streams int) error {
	temporary, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	var stderr bytes.Buffer
	cmd := exec.Command(command, "-0", fmt.Sprintf("-T%d", streams), "--stdout", "--", src)
	cmd.Stdout = temporary
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		temporary.Close()
		return fmt.Errorf("run xz: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, dst)
}

// compressXZFileWithConfig writes deterministic concatenated XZ streams.
// Byte-range stream boundaries are transparent to compliant readers.
func compressXZFileWithConfig(src, dst string, streams int, config xz.WriterConfig) error {
	if streams < 1 {
		return fmt.Errorf("xz streams must be positive")
	}
	if config.DictCap < 1 {
		return fmt.Errorf("xz dictionary size must be positive")
	}
	fIn, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fIn.Close()

	info, err := fIn.Stat()
	if err != nil {
		return err
	}
	if size := info.Size(); size < int64(streams) {
		streams = max(1, int(size))
	}
	if streams == 0 { // empty file: still emit one valid stream
		streams = 1
	}

	parts := make([]bytes.Buffer, streams)
	errs := make(chan error, streams)
	var wg sync.WaitGroup
	for i := range streams {
		start := info.Size() * int64(i) / int64(streams)
		end := info.Size() * int64(i+1) / int64(streams)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := config.NewWriter(&parts[i])
			if err != nil {
				errs <- err
				return
			}
			if _, err := io.Copy(w, io.NewSectionReader(fIn, start, end-start)); err != nil {
				_ = w.Close()
				errs <- err
				return
			}
			if err := w.Close(); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}

	fOut, err := os.Create(dst)
	if err != nil {
		return err
	}
	for i := range parts {
		if _, err := parts[i].WriteTo(fOut); err != nil {
			fOut.Close()
			return err
		}
	}
	return fOut.Close()
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
