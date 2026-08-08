package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

// xdbLookup is a minimal xdb searcher used to smoke-test generated files:
// it binary-searches the index segment using the header index pointers.
func xdbLookup(t *testing.T, path string, version xdbVersion, ip []byte) string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open xdb: %v", err)
	}
	defer f.Close()

	header := make([]byte, xdbHeaderLength)
	if _, err := io.ReadFull(f, header); err != nil {
		t.Fatalf("read header: %v", err)
	}
	if got := binary.LittleEndian.Uint16(header[0:]); got != xdbVersionNo {
		t.Fatalf("version no = %d, want %d", got, xdbVersionNo)
	}
	if got := binary.LittleEndian.Uint16(header[16:]); got != version.id {
		t.Fatalf("ip version = %d, want %d", got, version.id)
	}

	vectorOffset := (int(ip[0])*xdbVectorCols + int(ip[1])) * 8
	vector := make([]byte, 8)
	if _, err := f.ReadAt(vector, int64(xdbHeaderLength+vectorOffset)); err != nil {
		t.Fatalf("read vector index: %v", err)
	}
	lo := int64(binary.LittleEndian.Uint32(vector))
	hi := int64(binary.LittleEndian.Uint32(vector[4:]))
	if lo == 0 || hi == 0 {
		return ""
	}
	item := make([]byte, version.indexSize)

	for lo <= hi {
		mid := lo + (hi-lo)/int64(version.indexSize)/2*int64(version.indexSize)
		if _, err := f.ReadAt(item, mid); err != nil {
			t.Fatalf("read index item: %v", err)
		}
		if compareStoredIP(version, ip, item[:version.ipLen]) < 0 {
			hi = mid - int64(version.indexSize)
			continue
		}
		if compareStoredIP(version, ip, item[version.ipLen:2*version.ipLen]) > 0 {
			lo = mid + int64(version.indexSize)
			continue
		}
		dataLen := binary.LittleEndian.Uint16(item[2*version.ipLen:])
		dataPtr := binary.LittleEndian.Uint32(item[2*version.ipLen+2:])
		region := make([]byte, dataLen)
		if _, err := f.ReadAt(region, int64(dataPtr)); err != nil {
			t.Fatalf("read region: %v", err)
		}
		return string(region)
	}
	return ""
}

// compareStoredIP mirrors the official v3 searcher: query IPs are in network
// byte order, while IPv4 binary-index values retain the legacy little-endian
// encoding and IPv6 values use network byte order.
func compareStoredIP(version xdbVersion, query, stored []byte) int {
	if version.id == 4 {
		for i, b := range query {
			s := stored[len(stored)-1-i]
			if b < s {
				return -1
			}
			if b > s {
				return 1
			}
		}
		return 0
	}
	return bytes.Compare(query, stored)
}

func mustIP(t *testing.T, s string) []byte {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse ip %s: %v", s, err)
	}
	if addr.Is4() {
		b := addr.As4()
		return b[:]
	}
	b := addr.As16()
	return b[:]
}

func mustSegment(t *testing.T, cidr, region string) xdbSegment {
	t.Helper()
	seg, _, err := parseSegment(cidr, region)
	if err != nil {
		t.Fatalf("parseSegment(%s): %v", cidr, err)
	}
	return seg
}

func TestXdbIPv4Smoke(t *testing.T) {
	segs := fillGaps([]xdbSegment{
		mustSegment(t, "1.0.0.0/24", "AU|OC|13335|Cloudflare"),
		mustSegment(t, "8.8.8.0/24", "US|NA|15169|Google LLC"),
		mustSegment(t, "9.0.0.0/8", "US|NA|0|"), // crosses second-byte boundaries
	}, xdbIPv4.ipLen)

	path := filepath.Join(t.TempDir(), "test.ipv4.xdb")
	if err := writeXdb(path, xdbIPv4, segs); err != nil {
		t.Fatalf("writeXdb: %v", err)
	}

	cases := map[string]string{
		"1.0.0.0":         "AU|OC|13335|Cloudflare",
		"1.0.0.255":       "AU|OC|13335|Cloudflare",
		"8.8.8.8":         "US|NA|15169|Google LLC",
		"9.1.2.3":         "US|NA|0|",
		"9.255.255.255":   "US|NA|0|",
		"2.2.2.2":         "", // gap segment
		"0.0.0.0":         "", // leading gap
		"255.255.255.255": "", // trailing gap
	}
	for ip, want := range cases {
		if got := xdbLookup(t, path, xdbIPv4, mustIP(t, ip)); got != want {
			t.Errorf("lookup(%s) = %q, want %q", ip, got, want)
		}
	}
}

func TestXdbIPv6Smoke(t *testing.T) {
	segs := fillGaps([]xdbSegment{
		mustSegment(t, "2001:4860:4860::/48", "US|NA|15169|Google LLC"),
		mustSegment(t, "2401:b180:2800::/33", "CN|AS|0|Alibaba"),
	}, xdbIPv6.ipLen)

	path := filepath.Join(t.TempDir(), "test.ipv6.xdb")
	if err := writeXdb(path, xdbIPv6, segs); err != nil {
		t.Fatalf("writeXdb: %v", err)
	}

	cases := map[string]string{
		"2001:4860:4860::8888": "US|NA|15169|Google LLC",
		"2401:b180:2800::1":    "CN|AS|0|Alibaba",
		"::1":                  "",
		"ffff::1":              "",
	}
	for ip, want := range cases {
		if got := xdbLookup(t, path, xdbIPv6, mustIP(t, ip)); got != want {
			t.Errorf("lookup(%s) = %q, want %q", ip, got, want)
		}
	}
}

func TestParseSegment(t *testing.T) {
	seg, version, err := parseSegment("1.2.3.0/24", "R")
	if err != nil {
		t.Fatalf("parseSegment: %v", err)
	}
	if version.id != 4 {
		t.Fatalf("version = %d, want 4", version.id)
	}
	if !bytes.Equal(seg.start, []byte{1, 2, 3, 0}) || !bytes.Equal(seg.end, []byte{1, 2, 3, 255}) {
		t.Errorf("range = %v-%v, want 1.2.3.0-1.2.3.255", seg.start, seg.end)
	}

	// Plain IP addresses are treated as single-host prefixes.
	seg, _, err = parseSegment("1.7.168.174", "R")
	if err != nil {
		t.Fatalf("parseSegment plain IP: %v", err)
	}
	if !bytes.Equal(seg.start, seg.end) || !bytes.Equal(seg.start, []byte{1, 7, 168, 174}) {
		t.Errorf("plain IP range = %v-%v, want 1.7.168.174-1.7.168.174", seg.start, seg.end)
	}

	seg, _, err = parseSegment("1.2.3.4/0", "R")
	if err != nil {
		t.Fatalf("parseSegment /0: %v", err)
	}
	if !bytes.Equal(seg.start, []byte{0, 0, 0, 0}) || !bytes.Equal(seg.end, []byte{255, 255, 255, 255}) {
		t.Errorf("/0 range = %v-%v, want full space", seg.start, seg.end)
	}
}

func TestSegmentSplit(t *testing.T) {
	seg := mustSegment(t, "1.0.0.0/15", "R") // 1.0.0.0 - 1.1.255.255
	parts := seg.split()
	if len(parts) != 2 {
		t.Fatalf("split into %d parts, want 2", len(parts))
	}
	if !bytes.Equal(parts[0].end, []byte{1, 0, 255, 255}) {
		t.Errorf("parts[0].end = %v, want 1.0.255.255", parts[0].end)
	}
	if !bytes.Equal(parts[1].start, []byte{1, 1, 0, 0}) {
		t.Errorf("parts[1].start = %v, want 1.1.0.0", parts[1].start)
	}
	for i, p := range parts {
		if p.start[0] != p.end[0] || p.start[1] != p.end[1] {
			t.Errorf("part %d crosses a two-byte boundary: %v-%v", i, p.start, p.end)
		}
	}
}

func TestIncDecIP(t *testing.T) {
	if got := incIP([]byte{1, 2, 3, 255}); !bytes.Equal(got, []byte{1, 2, 4, 0}) {
		t.Errorf("incIP = %v, want 1.2.4.0", got)
	}
	if got := incIP([]byte{255, 255, 255, 255}); !bytes.Equal(got, []byte{0, 0, 0, 0}) {
		t.Errorf("incIP overflow = %v, want wrap to 0.0.0.0", got)
	}
	if got := decIP([]byte{1, 2, 4, 0}); !bytes.Equal(got, []byte{1, 2, 3, 255}) {
		t.Errorf("decIP = %v, want 1.2.3.255", got)
	}
}

func TestFillGaps(t *testing.T) {
	segs := fillGaps([]xdbSegment{
		mustSegment(t, "8.8.8.0/24", "A"),
		mustSegment(t, "1.0.1.0/24", "B"),
		mustSegment(t, "1.0.0.0/24", "B"), // unsorted on purpose
	}, xdbIPv4.ipLen)

	// Verify full continuous coverage.
	if !bytes.Equal(segs[0].start, []byte{0, 0, 0, 0}) {
		t.Fatalf("first segment starts at %v, want 0.0.0.0", segs[0].start)
	}
	if !bytes.Equal(segs[len(segs)-1].end, []byte{255, 255, 255, 255}) {
		t.Fatalf("last segment ends at %v, want 255.255.255.255", segs[len(segs)-1].end)
	}
	for i := 1; i < len(segs); i++ {
		if want := incIP(segs[i-1].end); !bytes.Equal(segs[i].start, want) {
			t.Fatalf("gap/overlap between segment %d and %d", i-1, i)
		}
	}
	if !bytes.Equal(segs[1].start, []byte{1, 0, 0, 0}) || !bytes.Equal(segs[1].end, []byte{1, 0, 1, 255}) {
		t.Fatalf("equal adjacent regions were not coalesced: %v-%v", segs[1].start, segs[1].end)
	}
}
