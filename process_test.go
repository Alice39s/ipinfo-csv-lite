package main

import (
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
)

func TestParseASN(t *testing.T) {
	cases := map[string]int{
		"AS13335":   13335,
		"AS15169":   15169,
		" AS6939 ":  6939,
		"":          0,
		"   ":       0,
		"AS":        0,
		"ASabc":     0,
		"foo":       0,
		"0":         0,
		"13335":     13335, // no prefix is still accepted
		"AS123AS45": 0,     // only a single leading prefix is stripped; the rest must be a clean integer
	}
	for in, want := range cases {
		if got := parseASN(in); got != want {
			t.Errorf("parseASN(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTransformRow(t *testing.T) {
	t.Run("valid row", func(t *testing.T) {
		row := []string{"1.0.0.0/24", "Australia", "AU", "Oceania", "OC", "AS13335", "Cloudflare", "cloudflare.com"}
		want := []string{"1.0.0.0/24", "AU", "OC", "13335", "Cloudflare"}
		got, ok := transformRow(row)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("empty asn becomes 0", func(t *testing.T) {
		row := []string{"57.144.0.0/16", "United States", "US", "North America", "NA", "", "", ""}
		got, ok := transformRow(row)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got[3] != "0" || got[4] != "" {
			t.Errorf("got %v, want as_number=0 and empty as_name", got)
		}
	})

	t.Run("short row skipped", func(t *testing.T) {
		if _, ok := transformRow([]string{"bad", "row"}); ok {
			t.Error("expected ok=false for row with fewer than 7 columns")
		}
	})

	t.Run("empty network skipped", func(t *testing.T) {
		row := []string{"  ", "France", "FR", "Europe", "EU", "AS0", "Empty", "example.fr"}
		if _, ok := transformRow(row); ok {
			t.Error("expected ok=false for empty network")
		}
	})
}

func TestTransformCSVOrder(t *testing.T) {
	// Build an input spanning many chunks and use few workers, so a
	// dispatch/collect deadlock would hang this test (runtime panics).
	const total = 250000
	var sb strings.Builder
	sb.WriteString("ignored-header\n") // consumed by caller, not transformCSV
	for i := range total {
		row := []string{"1.0.0.0/24", "Australia", "AU", "Oceania", "OC", "AS13335", "Cloudflare", "cloudflare.com"}
		if i%10 == 0 {
			row = []string{"bad", "row"}
		}
		sb.WriteString(strings.Join(row, ","))
		sb.WriteString("\n")
	}

	reader := csv.NewReader(strings.NewReader(sb.String()))
	reader.FieldsPerRecord = -1
	reader.Read() // skip header like runProcess does

	var out strings.Builder
	writer := csv.NewWriter(&out)
	if err := transformCSV(reader, writer, 2); err != nil {
		t.Fatalf("transformCSV: %v", err)
	}
	writer.Flush()

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != total-total/10 {
		t.Fatalf("got %d rows, want %d", len(lines), total-total/10)
	}
	for i, line := range lines {
		if line != "1.0.0.0/24,AU,OC,13335,Cloudflare" {
			t.Fatalf("row %d mismatch: %q", i, line)
		}
	}
}
