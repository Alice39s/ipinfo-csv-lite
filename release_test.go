package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestCompressXZFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.csv")
	dst := filepath.Join(dir, "output.csv.xz")
	want := bytes.Repeat([]byte("1.0.0.0/24,AU,OC,13335,Cloudflare\n"), 1000)
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := compressXZFile(src, dst, 4); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := xz.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decompressed content differs: got %d bytes, want %d", len(got), len(want))
	}
}

func TestCompressXZFilePureGoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.csv")
	dst := filepath.Join(dir, "output.csv.xz")
	want := bytes.Repeat([]byte("2001:db8::/32,ZZ,XX,0,Example\n"), 1000)
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatal(err)
	}
	config := xz.WriterConfig{DictCap: xzDictionarySize}
	if err := compressXZFileWithConfig(src, dst, 4, config); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := xz.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decompressed content differs: got %d bytes, want %d", len(got), len(want))
	}
}

func TestCompressXZFileRejectsInvalidStreamCount(t *testing.T) {
	if err := compressXZFile("unused", "unused", 0); err == nil {
		t.Fatal("expected invalid stream count error")
	}
}
