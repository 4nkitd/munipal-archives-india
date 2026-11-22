package tests

import (
	"database/sql"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/municipal-archives-india/manesar/internal/indexer"
	"github.com/municipal-archives-india/manesar/internal/model"
	"github.com/municipal-archives-india/manesar/internal/search"
)

func TestLiveIndexRelevanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("live neural benchmark disabled in short mode")
	}
	data := filepath.Join("..", "data")
	rawPointer, err := os.ReadFile(filepath.Join(data, indexer.CurrentFilename))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("no published CURRENT generation")
	}
	check(t, err)
	gen := strings.TrimSpace(string(rawPointer))
	if filepath.Dir(gen) != indexer.GenerationsDir || !filepath.IsLocal(gen) {
		t.Fatalf("invalid CURRENT pointer %q", gen)
	}
	base := filepath.Join(data, gen)
	db, vectors := filepath.Join(base, indexer.SQLiteFilename), filepath.Join(base, indexer.VectorDirname)
	stat := func(path string) fs.FileInfo {
		t.Helper()
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("live index not built: %s", path)
		}
		check(t, err)
		return info
	}
	// BuildIndex publishes vectors by directory rename. Reject a changing generation
	// or uncheckpointed SQLite database instead of opening a writer on the live DB.
	checkpointed := func() {
		t.Helper()
		i, err := os.Stat(db + "-wal")
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		check(t, err)
		if i.Size() != 0 {
			t.Skip("live index has uncheckpointed writes")
		}
	}
	stat(filepath.Join(base, indexer.CompleteMarker))
	firstDB, firstVectors := stat(db), stat(vectors)
	checkpointed()
	dir := t.TempDir()
	check(t, os.CopyFS(filepath.Join(dir, indexer.VectorDirname), os.DirFS(vectors)))
	raw, err := os.ReadFile(db)
	check(t, err)
	check(t, os.WriteFile(filepath.Join(dir, indexer.SQLiteFilename), raw, 0600))
	checkpointed()
	lastDB := stat(db)
	if !os.SameFile(firstVectors, stat(vectors)) || !os.SameFile(firstDB, lastDB) || !firstDB.ModTime().Equal(lastDB.ModTime()) || firstDB.Size() != lastDB.Size() {
		t.Skip("live index changed while snapshotting")
	}
	models := t.TempDir()
	cache := filepath.Join("..", ".cache", "hugot-models", "BAAI_bge-small-en-v1.5")
	if _, err := os.Stat(cache); err == nil {
		check(t, os.CopyFS(filepath.Join(models, filepath.Base(cache)), os.DirFS(cache)))
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	emb, err := indexer.NewHugotEmbedder(t.Context(), models)
	check(t, err)
	t.Cleanup(func() { check(t, emb.Close()) })
	u := url.URL{Scheme: "file", Path: filepath.Join(dir, indexer.SQLiteFilename), RawQuery: "mode=ro&immutable=1"}
	snapshot, err := sql.Open("sqlite", u.String())
	check(t, err)
	t.Cleanup(func() { check(t, snapshot.Close()) })
	v, err := indexer.OpenVectorDB(filepath.Join(dir, indexer.VectorDirname))
	check(t, err)
	e, err := search.NewEngine(snapshot, v, emb)
	check(t, err)
	sum := 0
	usedVector := false
	queries := []struct{ city, q, ids string }{
		{"manesar", "How do I correct my property ID in Manesar?", "mcm-svc-property-id-objection-guide mcm-tax-portal-guide"},
		{"gurugram", "What is MCG doing about waste at Bandhwari?", "mcg-tenders-bandhwari-swm"},
		{"faridabad", "Which MCF helpline takes sanitation complaints?", "mcf-officers-helplines mcf-svc-grievances-certs"},
		{"sohna", "Who plans the industrial corridor around IMT Sohna?", "sohna-plan-imt-corridor"},
		{"rewari", "Is my colony in Rewari on the list of approved colonies?", "rewari-notif-colonies"},
		{"bhiwadi", "What pollution restrictions apply to factories in Bhiwadi?", "bhiwadi-notif-grap-cetp"},
		{"sonipat", "Where can I find Sonipat's municipal budget?", "sonipat-fin-budgets"},
		{"panipat", "Who was elected to the Panipat council in 2025?", "panipat-gov-council-2025"},
		{"panchkula", "How can I file an RTI request in Panchkula?", "panchkula-svc-rti-rts"},
		{"rohtak", "Who can I contact at the Rohtak municipal office?", "rohtak-officers-directory"},
		{"karnal", "Which works are part of the Karnal Smart City plan?", "karnal-plan-smartcity-swm karnal-tenders-works"},
		{"hisar", "Where can I check property tax rules in Hisar?", "hisar-ref-tax-planning"},
		{"ambala", "How does the ring road fit into Ambala's development plan?", "ambala-plan-ringroad-swm"},
		{"yamunanagar", "How is industrial waste managed in Yamunanagar?", "yamunanagar-plan-industrial-swm"},
		{"bahadurgarh", "How does the metro connect Bahadurgarh to the KMP corridor?", "bahadurgarh-plan-kmp-metro"},
		{"palwal", "What does the Haryana Orbital Rail Corridor mean for Palwal?", "palwal-plan-horc-kmp"},
		{"jhajjar", "Which authority handles the MET industrial corridor in Jhajjar?", "jhajjar-plan-met-corridor"},
		{"nuh", "Where can a contractor find Nuh municipal tenders?", "nuh-tenders-works"},
		{"dharuhera", "How do I pay municipal property tax in Dharuhera?", "dharuhera-svc-tax-rti"},
		{"farrukhnagar", "What eco-sensitive zone restrictions apply near Farrukhnagar?", "farrukhnagar-plan-kmp-esz"},
		{"pataudi-jatoli-mandi", "Which wards are part of Pataudi-Jatoli Mandi?", "pataudi-wards-geography"},
		{"manesar", "Who is the junior engineer for ward 9?", "mcm-officers-ward-engineering-matrix mcm-officers-complete-directory"},
		{"gurugram", "Who handles stormwater drainage work for MCG?", "mcg-tenders-water-drainage mcg-officers-zone-engineers"},
		{"all", "Which tenders cover Bandhwari waste disposal in Gurugram and Faridabad?", "mcg-tenders-bandhwari-swm mcf-tenders-swm-sanitation"},
		{"all", "Where are the KMP corridor plans for Palwal and Bahadurgarh?", "palwal-plan-horc-kmp bahadurgarh-plan-kmp-metro"},
	}
	for _, tc := range queries {
		start := time.Now()
		r, err := e.Search(t.Context(), model.SearchRequest{Municipality: tc.city, Mode: model.SearchModeHybrid, Limit: 3, Query: tc.q})
		check(t, err)
		elapsed := time.Since(start)
		usedVector = usedVector || slices.ContainsFunc(r.Hits, func(h model.SearchHit) bool { return slices.Contains(h.MatchSources, "vector") })
		var ids []string
		n := 0
		for _, hit := range r.Hits {
			ids = append(ids, hit.DocID)
			if tc.city != "all" && hit.Municipality != tc.city {
				t.Errorf("%s query returned a document from %s", tc.city, hit.Municipality)
			}
			if slices.Contains(strings.Fields(tc.ids), hit.DocID) {
				n++
			}
		}
		t.Logf("%q precision@3=%.3f latency=%s docs=%v", tc.q, float64(n)/3, elapsed, ids)
		sum += n
	}
	if !usedVector {
		t.Error("no benchmark result had a neural match")
	}
	t.Logf("mean precision@3=%.3f (%d queries)", float64(sum)/float64(3*len(queries)), len(queries))
}
