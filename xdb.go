package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// XDB format constants (ip2region xdb v3 layout):
//
//	| Header 256B | Vector index 512 KiB | Region data | Binary index |
//
// All multi-byte integer fields are little-endian; IP byte arrays stay in
// big-endian network order.
const (
	xdbVersionNo     = 2
	xdbIndexPolicy   = 1 // vector index cache policy
	xdbHeaderLength  = 256
	xdbVectorCols    = 256
	xdbVectorLength  = xdbVectorCols * xdbVectorCols * 8 // 512 KiB
	xdbMaxRegionSize = 0xFFFF
)

// xdbVersion describes the per-IP-version layout parameters.
type xdbVersion struct {
	id        uint16 // 4 or 6
	ipLen     int    // 4 or 16 bytes
	indexSize int    // bytes per binary index item
}

var (
	xdbIPv4 = xdbVersion{id: 4, ipLen: 4, indexSize: 4 + 4 + 2 + 4}
	xdbIPv6 = xdbVersion{id: 6, ipLen: 16, indexSize: 16 + 16 + 2 + 4}
)

// xdbSegment is one continuous IP range mapped to a region string.
type xdbSegment struct {
	start, end []byte // big-endian IP bytes, len == version.ipLen
	region     string
}

// runXdb converts data/ipinfo-lite.csv into ip2region xdb files:
// data/ipinfo-lite.ipv4.xdb and data/ipinfo-lite.ipv6.xdb. The region string
// stored for each segment is "country_code|continent_code|as_number|as_name".
func runXdb() error {
	input := filepath.Join(dataDir, "ipinfo-lite.csv")

	fIn, reader, err := openLiteCSV(input)
	if err != nil {
		return err
	}
	defer fIn.Close()

	var v4Segs, v6Segs []xdbSegment
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(row) < 5 || row[0] == "" {
			continue
		}

		seg, version, err := parseSegment(row[0], row[1]+"|"+row[2]+"|"+row[3]+"|"+row[4])
		if err != nil {
			continue // skip unparsable CIDRs
		}
		if version.id == 4 {
			v4Segs = append(v4Segs, seg)
		} else {
			v6Segs = append(v6Segs, seg)
		}
	}

	// IPv4 and IPv6 files are independent; build them concurrently.
	jobs := []struct {
		name    string
		version xdbVersion
		segs    []xdbSegment
	}{
		{"ipinfo-lite.ipv4.xdb", xdbIPv4, v4Segs},
		{"ipinfo-lite.ipv6.xdb", xdbIPv6, v6Segs},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(jobs))
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output := filepath.Join(dataDir, job.name)
			segs := fillGaps(job.segs, job.version.ipLen)
			if err := writeXdb(output, job.version, segs); err != nil {
				errs <- err
				return
			}
			fmt.Printf("Wrote %s (%d segments)\n", output, len(segs))
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

// parseSegment converts a CIDR into an xdbSegment covering the full prefix range.
// Plain IP addresses (treated as /32 or /128) are accepted since they occur
// in the source data.
func parseSegment(cidr, region string) (xdbSegment, xdbVersion, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		addr, addrErr := netip.ParseAddr(cidr)
		if addrErr != nil {
			return xdbSegment{}, xdbVersion{}, err
		}
		bits := 128
		if addr.Is4() {
			bits = 32
		}
		p = netip.PrefixFrom(addr, bits)
	}
	p = p.Masked()

	version := xdbIPv6
	var start []byte
	if p.Addr().Is4() {
		version = xdbIPv4
		b := p.Addr().As4()
		start = b[:]
	} else {
		b := p.Addr().As16()
		start = b[:]
	}

	end := make([]byte, version.ipLen)
	copy(end, start)
	for i := p.Bits(); i < version.ipLen*8; i++ {
		end[i/8] |= 1 << (7 - uint(i%8))
	}

	return xdbSegment{start: start, end: end, region: region}, version, nil
}

// fillGaps sorts segments and inserts empty-region segments so the result is
// continuous and covers the entire address space.
func fillGaps(segs []xdbSegment, ipLen int) []xdbSegment {
	slices.SortFunc(segs, func(a, b xdbSegment) int {
		return bytes.Compare(a.start, b.start)
	})

	maxIP := bytes.Repeat([]byte{0xff}, ipLen)
	var out []xdbSegment
	prev := make([]byte, ipLen) // next expected start, all zeros initially
	for _, s := range segs {
		if prev == nil || bytes.Compare(s.end, prev) < 0 {
			continue // fully overlapped by a previous segment
		}
		if bytes.Compare(s.start, prev) < 0 {
			s.start = prev // clamp partial overlap
		}
		if bytes.Compare(s.start, prev) > 0 {
			gapEnd := make([]byte, ipLen)
			copy(gapEnd, s.start)
			decIPInPlace(gapEnd)
			out = append(out, xdbSegment{start: prev, end: gapEnd, region: ""})
		}
		out = append(out, s)
		if bytes.Equal(s.end, maxIP) {
			prev = nil // saturated: no more address space left
		} else {
			prev = make([]byte, ipLen)
			copy(prev, s.end)
			incIPInPlace(prev)
		}
	}
	if prev != nil {
		out = append(out, xdbSegment{start: prev, end: maxIP, region: ""})
	}
	return out
}

// writeXdb writes the full xdb file for one IP version.
// Segments must be continuous and cover the whole address space (see fillGaps).
func writeXdb(dst string, version xdbVersion, segs []xdbSegment) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	// All sequential writes go through a large buffer; the header and vector
	// index are patched afterwards with WriteAt once the buffer is flushed.
	w := bufio.NewWriterSize(f, 1<<20)

	// Header segment: 256 bytes, index pointers patched at the end.
	header := make([]byte, xdbHeaderLength)
	binary.LittleEndian.PutUint16(header[0:], xdbVersionNo)
	binary.LittleEndian.PutUint16(header[2:], xdbIndexPolicy)
	binary.LittleEndian.PutUint32(header[4:], uint32(time.Now().Unix()))
	binary.LittleEndian.PutUint16(header[16:], version.id)
	binary.LittleEndian.PutUint16(header[18:], 4) // runtime pointer bytes
	if _, err := w.Write(header); err != nil {
		f.Close()
		return err
	}

	// Vector index segment: kept in memory, filled during index writing,
	// flushed afterwards; write the zeroed placeholder now.
	vectorIndex := make([]byte, xdbVectorLength)
	if _, err := w.Write(vectorIndex); err != nil {
		f.Close()
		return err
	}

	offset := int64(xdbHeaderLength + xdbVectorLength)

	// Region data segment: deduplicated, each unique region written once.
	regionPool := make(map[string]uint32)
	for _, seg := range segs {
		if _, ok := regionPool[seg.region]; ok {
			continue
		}
		if len(seg.region) > xdbMaxRegionSize {
			f.Close()
			return fmt.Errorf("region too long (%d bytes): %q", len(seg.region), seg.region)
		}
		// WriteString avoids the []byte(seg.region) conversion alloc per region.
		if _, err := w.WriteString(seg.region); err != nil {
			f.Close()
			return err
		}
		regionPool[seg.region] = uint32(offset)
		offset += int64(len(seg.region))
	}

	// Binary index segment: split segments on two-byte boundaries so the
	// vector index can point at exact sub-ranges.
	var startIndexPtr, endIndexPtr int64 = -1, -1
	item := make([]byte, version.indexSize)
	for _, seg := range segs {
		ptr, ok := regionPool[seg.region]
		if !ok {
			f.Close()
			return fmt.Errorf("missing ptr cache for region %q", seg.region)
		}

		for _, s := range seg.split() {
			copy(item[0:], s.start)
			copy(item[version.ipLen:], s.end)
			binary.LittleEndian.PutUint16(item[2*version.ipLen:], uint16(len(seg.region)))
			binary.LittleEndian.PutUint32(item[2*version.ipLen+2:], ptr)
			if _, err := w.Write(item); err != nil {
				f.Close()
				return err
			}

			setVectorIndex(vectorIndex, s.start, uint32(offset), version.indexSize)
			endIndexPtr = offset
			if startIndexPtr == -1 {
				startIndexPtr = offset
			}
			offset += int64(version.indexSize)
		}
	}

	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}

	// Flush the vector index buffer.
	if _, err := f.WriteAt(vectorIndex, xdbHeaderLength); err != nil {
		f.Close()
		return err
	}

	// Patch the binary index start/end pointers into the header.
	ptrBuf := make([]byte, 8)
	binary.LittleEndian.PutUint32(ptrBuf[0:], uint32(startIndexPtr))
	binary.LittleEndian.PutUint32(ptrBuf[4:], uint32(endIndexPtr))
	if _, err := f.WriteAt(ptrBuf, 8); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

// split breaks a segment into sub-segments whose start and end IPs share the
// same first two bytes, so each one maps to exactly one vector index cell.
func (s xdbSegment) split() []xdbSegment {
	if s.start[0] == s.end[0] && s.start[1] == s.end[1] {
		return []xdbSegment{s}
	}

	var out []xdbSegment
	cur := s.start
	for {
		if cur[0] == s.end[0] && cur[1] == s.end[1] {
			out = append(out, xdbSegment{start: cur, end: s.end, region: s.region})
			return out
		}
		blockEnd := make([]byte, len(cur))
		blockEnd[0], blockEnd[1] = cur[0], cur[1]
		for i := 2; i < len(blockEnd); i++ {
			blockEnd[i] = 0xff
		}
		out = append(out, xdbSegment{start: cur, end: blockEnd, region: s.region})
		cur = incIP(blockEnd)
	}
}

// setVectorIndex records the binary index range for the vector cell of ip.
func setVectorIndex(vectorIndex []byte, ip []byte, ptr uint32, indexSize int) {
	idx := (int(ip[0])*xdbVectorCols + int(ip[1])) * 8
	if binary.LittleEndian.Uint32(vectorIndex[idx:]) == 0 {
		binary.LittleEndian.PutUint32(vectorIndex[idx:], ptr)
		binary.LittleEndian.PutUint32(vectorIndex[idx+4:], ptr+uint32(indexSize))
	} else {
		binary.LittleEndian.PutUint32(vectorIndex[idx+4:], ptr+uint32(indexSize))
	}
}

// incIP returns ip + 1 (wraps on overflow, callers check for saturation).
func incIP(ip []byte) []byte {
	out := make([]byte, len(ip))
	copy(out, ip)
	incIPInPlace(out)
	return out
}

// incIPInPlace mutates ip to ip + 1 (wraps on overflow).
func incIPInPlace(ip []byte) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

// decIP returns ip - 1 (wraps on underflow).
func decIP(ip []byte) []byte {
	out := make([]byte, len(ip))
	copy(out, ip)
	decIPInPlace(out)
	return out
}

// decIPInPlace mutates ip to ip - 1 (wraps on underflow).
func decIPInPlace(ip []byte) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]--
		if ip[i] != 0xff {
			break
		}
	}
}
