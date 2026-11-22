package tests

import (
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/municipal-archives-india/manesar/internal/indexer"
)

func TestIndiaCorporationCoverage(t *testing.T) {
	expected := map[string]string{}
	states := map[string]bool{}
	validSlug := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	for _, table := range []struct {
		path string
		cols int
		want int
	}{
		{"../docs/india-roster.tsv", 3, 279},
		{"../docs/retained-councils.tsv", 2, 10},
	} {
		file, err := os.ReadFile(table.path)
		check(t, err)
		lines := strings.Split(strings.TrimSuffix(string(file), "\n"), "\n")
		before := len(expected)
		for i, line := range lines[1:] {
			fields := strings.Split(line, "\t")
			if len(fields) != table.cols || fields[0] == "" {
				t.Fatalf("%s:%d: invalid roster row", table.path, i+2)
			}
			state := fields[0]
			if table.cols == 3 {
				source, err := url.Parse(fields[1])
				if err != nil || source.Scheme != "https" || source.Hostname() == "" || states[state] {
					t.Fatalf("%s:%d: invalid or duplicated jurisdiction evidence", table.path, i+2)
				}
				states[state] = true
			} else if !states[state] {
				t.Fatalf("%s:%d: unknown state %q", table.path, i+2, state)
			}
			for _, slug := range strings.Fields(fields[table.cols-1]) {
				if !validSlug.MatchString(slug) || expected[slug] != "" {
					t.Fatalf("%s:%d: invalid or duplicated slug %q", table.path, i+2, slug)
				}
				expected[slug] = state
			}
		}
		if n := len(expected) - before; n != table.want {
			t.Fatalf("%s: %d slugs, want %d", table.path, n, table.want)
		}
	}
	if len(states) != 36 {
		t.Fatalf("roster has %d states and UTs, want 36", len(states))
	}
	profiles, err := indexer.ScanMunicipalities("../archives")
	check(t, err)
	for _, p := range profiles {
		if expected[p.Slug] != p.State || p.State == "" {
			t.Errorf("%s: state %q, expected %q", p.Slug, p.State, expected[p.Slug])
		}
		delete(expected, p.Slug)
	}
	for slug, state := range expected {
		t.Errorf("missing archive: %s (%s)", slug, state)
	}
	t.Logf("reconciled %d archive profiles with 279 corporations and 10 retained councils across 36 audited jurisdictions", len(profiles))
}
