package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"math"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/municipal-archives-india/manesar/internal/indexer"
	"github.com/municipal-archives-india/manesar/internal/model"
	"github.com/municipal-archives-india/manesar/internal/search"
	"github.com/municipal-archives-india/manesar/internal/server"
	"github.com/municipal-archives-india/manesar/internal/web"
)

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseAllManesarArchives(t *testing.T) {
	root := "../archives"
	dir := filepath.Join(root, "manesar")
	profiles, err := indexer.ScanMunicipalities(root)
	check(t, err)
	if !slices.ContainsFunc(profiles, func(p model.MunicipalityProfile) bool { return p.Slug == "manesar" && p.WardsCount == 20 }) {
		t.Fatal("missing 20-ward Manesar profile")
	}
	categories := strings.Fields("citizen-services finance-legal governance notifications-orders officers-directory reference-documents tenders-works urban-planning-imt wards-geography")
	ids := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		d, err := indexer.ParseMarkdownFile(path, root)
		if err != nil {
			t.Error(err)
			return nil
		}
		if before, ok := ids[d.ID]; ok {
			t.Errorf("%s duplicates id %q from %s", path, d.ID, before)
		}
		ids[d.ID] = path
		if d.Municipality != "manesar" || !slices.Contains(categories, d.Category) || strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.SourceURL) == "" || strings.TrimSpace(d.Summary) == "" || len(indexer.ChunkDocument(d)) == 0 {
			t.Errorf("%s: invalid document metadata or empty chunks", path)
		}
		return nil
	})
	check(t, err)
	if len(ids) == 0 {
		t.Fatal("no documents parsed")
	}
	ws, err := indexer.LoadMunicipalityWards(dir)
	check(t, err)
	oset, err := indexer.LoadMunicipalityOfficers(dir)
	check(t, err)
	seen := map[int]bool{}
	for _, w := range ws {
		if w.WardNumber < 1 || w.WardNumber > 20 || seen[w.WardNumber] {
			t.Errorf("invalid or duplicate ward %d", w.WardNumber)
		}
		seen[w.WardNumber] = true
		if w.Councillor == nil || w.Councillor.Name == "" {
			t.Errorf("ward %d missing councillor", w.WardNumber)
		}
		for role, c := range map[string]*model.ContactInfo{"executive engineer": w.ExecutiveEngineer, "assistant engineer": w.AssistantEngineer, "junior engineer": w.JuniorEngineer} {
			if c == nil || !slices.ContainsFunc(oset, func(o model.OfficerRecord) bool {
				return c.Name != "" && c.Name == o.Name && strings.HasPrefix(strings.ToLower(o.Designation), role) && slices.Contains(o.Wards, w.WardNumber)
			}) {
				t.Errorf("ward %d missing assigned %s", w.WardNumber, role)
			}
		}
	}
	if len(seen) != 20 {
		t.Errorf("loaded %d distinct wards", len(seen))
	}
	t.Logf("parsed %d documents, %d wards, %d officers", len(ids), len(ws), len(oset))
}

type mockEmbedder struct{}

func TestParseAllMunicipalities(t *testing.T) {
	root := "../archives"
	profiles, err := indexer.ScanMunicipalities(root)
	check(t, err)
	if len(profiles) < 21 {
		t.Fatalf("got %d profiles, want at least the original 21", len(profiles))
	}
	ids := map[[2]string]string{}
	wardCount, officerCount := 0, 0
	engineers := map[int][]model.ContactInfo{}
	for _, p := range profiles {
		dir := filepath.Join(root, p.Slug)
		before := len(ids)
		check(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			d, err := indexer.ParseMarkdownFile(path, root)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				return nil
			}
			if d.Municipality != p.Slug || d.ID == "" {
				t.Errorf("%s: unexpected municipality/id %q/%q", path, d.Municipality, d.ID)
			}
			key := [2]string{d.Municipality, d.ID}
			if other, ok := ids[key]; ok {
				t.Errorf("%s and %s have the same key %v", path, other, key)
			}
			ids[key] = path
			return nil
		}))
		ws, err := indexer.LoadMunicipalityWards(dir)
		check(t, err)
		oset, err := indexer.LoadMunicipalityOfficers(dir)
		check(t, err)
		if before == len(ids) {
			t.Errorf("%s: no documents", p.Slug)
		}
		wardCount += len(ws)
		officerCount += len(oset)
		if p.Slug == "manesar" {
			for _, w := range ws {
				engineers[w.WardNumber] = w.JuniorEngineers
			}
		}
		seenWards := map[int]bool{}
		for _, w := range ws {
			if w.WardNumber < 1 || seenWards[w.WardNumber] {
				t.Errorf("%s: invalid or duplicate ward number %d", p.Slug, w.WardNumber)
			}
			seenWards[w.WardNumber] = true
		}
	}
	for _, n := range []int{16, 17, 18} {
		jes := engineers[n]
		if len(jes) != 2 || jes[0].Name == "" || jes[1].Name == "" || jes[0].Name == jes[1].Name {
			t.Errorf("Manesar ward %d: want two distinct junior engineers, got %v", n, jes)
		}
	}
	t.Logf("parsed %d profiles, %d documents, %d wards, %d officers", len(profiles), len(ids), wardCount, officerCount)
	if len(ids) < 250 || wardCount < 450 || officerCount < 350 {
		t.Fatal("archive totals below regression thresholds")
	}
}

func TestNorthwestCorporationCoverage(t *testing.T) {
	raw, err := os.ReadFile("../docs/NORTHWEST_COVERAGE.md")
	check(t, err)
	_, roster, ok := strings.Cut(string(raw), "## Roster for reconciliation")
	if !ok {
		t.Fatal("northwest roster missing")
	}
	profiles, err := indexer.ScanMunicipalities("../archives")
	check(t, err)
	slugs := map[string]bool{}
	for _, p := range profiles {
		slugs[p.Slug] = true
	}
	targets := regexp.MustCompile("`([a-z0-9-]+)`").FindAllStringSubmatch(roster, -1)
	if len(targets) != 85 {
		t.Fatalf("got %d targets, want 85", len(targets))
	}
	seen := map[string]bool{}
	for _, target := range targets {
		if seen[target[1]] || !slugs[target[1]] {
			t.Errorf("duplicate or missing corporation %q", target[1])
		}
		seen[target[1]] = true
	}
	for _, slug := range strings.Fields("bahadurgarh bhiwadi dharuhera farrukhnagar jhajjar nuh palwal pataudi-jatoli-mandi rewari sohna") {
		if seen[slug] || !slugs[slug] {
			t.Errorf("missing or misclassified neighboring council/committee %q", slug)
		}
	}
	check(t, filepath.WalkDir("../archives", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !strings.EqualFold(filepath.Ext(path), ".md") && !strings.EqualFold(filepath.Ext(path), ".json") {
			t.Errorf("non-text archive file %s", path)
		}
		return err
	}))
}

func TestEntityAliasesAndCityDisambiguation(t *testing.T) {
	ws, err := indexer.LoadMunicipalityWards("../archives/manesar")
	check(t, err)
	for q, n := range map[string]int{"Bargujjar": 16, "Baskusla": 7, "Laknola": 15, "Rampur": 18, "Sikohpur": 17, "Ward 36": 36, "Ward 46": 46, "Ward 250": 250} {
		if got := search.DetectEntities(q, ws); !slices.Equal(got.Wards, []int{n}) {
			t.Errorf("%q: got %v, want ward %d", q, got, n)
		}
	}
	if got := search.DetectEntities("How do I correct my property ID in Manesar?", ws); len(got.Wards) != 0 || len(got.Villages) != 0 {
		t.Errorf("city incorrectly resolved as a village: %v", got)
	}
	local := []model.WardRecord{{WardNumber: 1, Villages: []string{"सहारनपुर"}}}
	if got := search.DetectEntities("सहारनपुर", local); !slices.Equal(got.Wards, []int{1}) || !slices.Equal(got.Villages, []string{"सहारनपुर"}) {
		t.Errorf("native-script locality was not resolved: %+v", got)
	}
}

func TestLocalScriptKeywordSearch(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	for _, d := range []struct{ slug, id, title, text string }{
		{"surat", "gujarati", "Gujarati example", "કરવેરા"},
		{"delhi", "hindi", "Hindi example", "किराया क़िला"},
	} {
		dir := filepath.Join(root, d.slug)
		check(t, os.Mkdir(dir, 0700))
		check(t, os.WriteFile(filepath.Join(dir, "municipality.json"), []byte(fmt.Sprintf(`{"slug":%q,"name":%q}`, d.slug, d.slug)), 0600))
		check(t, os.WriteFile(filepath.Join(dir, d.id+".md"), []byte(fmt.Sprintf("---\nid: %s\nmunicipality: %s\ntitle: %s\ncategory: governance\ndoc_type: notice\n---\n%s\n", d.id, d.slug, d.title, d.text)), 0600))
	}
	_, err := indexer.BuildIndex(t.Context(), root, data, mockEmbedder{})
	check(t, err)
	e := openEngine(t, data, mockEmbedder{})
	for _, tc := range []struct{ query, id string }{{"કરવેરા", "gujarati"}, {"કરવાર", ""}, {"किराया", "hindi"}, {"कराया", ""}, {"क़िला", "hindi"}, {"क़िला", "hindi"}} {
		r, err := e.Search(t.Context(), model.SearchRequest{Municipality: "all", Mode: model.SearchModeKeyword, Query: tc.query})
		check(t, err)
		if tc.id == "" && len(r.Hits) != 0 || tc.id != "" && (len(r.Hits) != 1 || r.Hits[0].DocID != tc.id) {
			t.Errorf("%q: hits=%+v; want %q", tc.query, r.Hits, tc.id)
		}
	}
	dbPath, _, ready := indexer.ResolveDataPaths(data)
	if !ready {
		t.Fatal("fresh index was rejected")
	}
	db, err := indexer.OpenSQLite(dbPath)
	check(t, err)
	_, err = db.Exec(`DELETE FROM index_meta WHERE key='fts_tokenizer'`)
	check(t, err)
	check(t, db.Close())
	if _, _, ready := indexer.ResolveDataPaths(data); ready {
		t.Error("old index should require a rebuild")
	}
}

func (mockEmbedder) ModelName() string { return "fixture-hash" }
func (mockEmbedder) Dimensions() int   { return 384 }
func (mockEmbedder) Close() error      { return nil }
func (mockEmbedder) EmbedQuery(ctx context.Context, s string) ([]float32, error) {
	v := make([]float32, 384)
	for _, word := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		h := fnv.New32a()
		h.Write([]byte(word))
		v[h.Sum32()%384]++
	}
	sum := float64(0)
	for _, x := range v {
		sum += float64(x * x)
	}
	if sum == 0 {
		v[0] = 1
	} else {
		for i := range v {
			v[i] /= float32(math.Sqrt(sum))
		}
	}
	return v, ctx.Err()
}
func (m mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, s := range texts {
		v, err := m.EmbedQuery(ctx, s)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func openEngine(t *testing.T, dir string, emb model.Embedder) *search.Engine {
	t.Helper()
	dbPath, vecDir, ready := indexer.ResolveDataPaths(dir)
	if !ready {
		t.Fatalf("no complete generation in %s", dir)
	}
	db, err := indexer.OpenSQLite(dbPath)
	check(t, err)
	t.Cleanup(func() { check(t, db.Close()) })
	v, err := indexer.OpenVectorDB(vecDir)
	check(t, err)
	engine, err := search.NewEngine(db, v, emb)
	check(t, err)
	return engine
}

const scriptTag = `<script>alert("fixture")</script>`

func fixture(t *testing.T) *search.Engine {
	t.Helper()
	root, dbDir := t.TempDir(), t.TempDir()
	for _, m := range []string{"manesar", "gurugram"} {
		dir := filepath.Join(root, m)
		check(t, os.Mkdir(dir, 0700))
		write := func(name, s string) { t.Helper(); check(t, os.WriteFile(filepath.Join(dir, name), []byte(s), 0600)) }
		write("municipality.json", fmt.Sprintf(`{"slug":%q,"name":%q,"state":"Haryana","district":"Gurugram"}`, m, m))
		if m == "manesar" {
			write("public-contacts.json", `[{"label":"Public duty desk","number":"01234567890","email":"contact@example.org","source_url":"https://example.org/contact"}]`)
			write("wards.json", `[{"ward_number":9,"name":"Bhangrola","villages":["Bhangrola"],"sectors":["83"],"source_url":"https://example.org/wards.pdf#page=9","verified_at":"2026-11-22"}]`)
			write("officers.json", `[{"sr_no":1,"name":"Asha Fixture","designation":"Junior Engineer","wards":[9]},{"sr_no":2,"name":"Leela Dummyfield","designation":"Commissioner","wards":[]}]`)
		} else {
			write("officers.json", `[{"sr_no":1,"name":"Leela Dummyfield","designation":"Commissioner","wards":[]}]`)
		}
		for _, d := range []struct{ id, title, metadata, body string }{
			{"pond", "Bhangrola pond tender", "wards: [9]\nvillages: [Bhangrola]\nsectors: ['83']\naction_url: https://etenders.hry.nic.in/pond", "Bhangrola pond tender bids for ward 9 must arrive within 14 days.\n\n" + scriptTag},
			{"drain", "Pond drainage works", "", "Repair the pond drainage outlet before monsoon."},
			{"tax", "Property tax", "", "Pay property tax after verifying the assessment."},
		} {
			write(d.id+".md", fmt.Sprintf("---\nid: %s-%s\nmunicipality: %s\ntitle: %s\ncategory: citizen-services\ndoc_type: notice\nsummary: %s\nsource_url: https://mcmanesar.in/%s\n%s\n---\n## Details\n%s", m, d.id, m, d.title, d.title, d.id, d.metadata, d.body))
		}
	}
	emb := mockEmbedder{}
	s, err := indexer.BuildIndex(t.Context(), root, dbDir, emb)
	check(t, err)
	if s.TotalDocuments != 6 || s.TotalChunks != 6 || s.MunicipalitiesIndexed != 2 {
		t.Fatalf("fixture stats: %+v", s)
	}
	return openEngine(t, dbDir, emb)
}

func TestHybridSearchAndMultiMunicipality(t *testing.T) {
	e := fixture(t)
	ctx := t.Context()
	for _, m := range []string{"manesar", "gurugram"} {
		for _, mode := range []model.SearchMode{model.SearchModeHybrid, model.SearchModeVector, model.SearchModeKeyword} {
			r, err := e.Search(ctx, model.SearchRequest{Municipality: m, Query: "Bhangrola pond tender", Mode: mode})
			check(t, err)
			if len(r.Hits) == 0 || r.Hits[0].DocID != m+"-pond" {
				t.Fatalf("%s/%s hits: %+v", m, mode, r.Hits)
			}
			src := r.Hits[0].MatchSources
			if slices.Contains(src, "keyword") != (mode != model.SearchModeVector) || slices.Contains(src, "vector") != (mode != model.SearchModeKeyword) {
				t.Errorf("%s match sources: %v", mode, src)
			}
			for _, h := range r.Hits {
				if h.Municipality != m || !strings.HasPrefix(h.DocID, m+"-") {
					t.Errorf("%s/%s leaked %s", m, mode, h.DocID)
				}
			}
		}
	}
	for _, q := range []string{"who is junior engineer for ward 9", "Bhangrola pond tender", "Sector 83"} {
		r, err := e.Search(ctx, model.SearchRequest{Municipality: "manesar", Query: q})
		check(t, err)
		x := r.DetectedEntities
		if !slices.Contains(x.Wards, 9) || q == "Bhangrola pond tender" && !slices.Contains(x.Villages, "Bhangrola") || q == "Sector 83" && !slices.Contains(x.Sectors, "83") {
			t.Errorf("%q entities: %+v", q, x)
		}
	}
	a, err := e.Ask(ctx, model.SearchRequest{Query: "who is junior engineer for ward 9", Municipality: "manesar"})
	check(t, err)
	b := a.Brief
	cited := func(i int) bool {
		return i > 0 && i <= len(b.Citations) && b.Citations[i-1].Index == i && b.Citations[i-1].SourceURL != ""
	}
	fromPond := func(i int) bool { return cited(i) && b.Citations[i-1].DocID == "manesar-pond" }
	if !strings.Contains(strings.Join(b.AnswerParagraphs, " "), "Asha Fixture") || !slices.ContainsFunc(b.Contacts, func(c model.CitedContact) bool { return c.Name == "Asha Fixture" && cited(c.CitationIndex) }) || !slices.ContainsFunc(b.Facts, func(f model.ExtractedFact) bool { return f.Value == "14 days" && fromPond(f.CitationIndex) }) || !slices.ContainsFunc(b.ActionLinks, func(l model.ActionLink) bool {
		return l.URL == "https://etenders.hry.nic.in/pond" && fromPond(l.CitationIndex)
	}) || len(b.FollowUpQueries) == 0 {
		t.Errorf("ungrounded brief: %+v", b)
	}
	for _, tc := range []struct{ prefix, kind, label string }{{"ward 9", "ward", "Ward 9"}, {"bhang", "village", "Bhangrola"}, {"asha", "officer", "Asha Fixture"}, {"property", "document", "Property tax"}} {
		rows, err := e.Suggest(ctx, "manesar", tc.prefix, 8)
		check(t, err)
		if !slices.ContainsFunc(rows, func(x model.SuggestItem) bool { return x.Type == tc.kind && x.Label == tc.label && x.Query != "" }) {
			t.Errorf("suggest(%q): %+v", tc.prefix, rows)
		}
	}
	_, err = e.Search(ctx, model.SearchRequest{Mode: model.SearchModeKeyword, Query: `ward:9 (property) *tax* "foo" OR AND --- ' ; DROP TABLE`})
	check(t, err)
	d, err := e.GetDocument(ctx, "manesar", "manesar-pond")
	check(t, err)
	if len(d.Headings) != 1 || d.Headings[0].Text != "Details" || d.Headings[0].ID == "" || len(d.Related) == 0 || d.Related[0].ID != "manesar-drain" || d.Related[0].Similarity <= 0 {
		t.Errorf("headings=%v related=%v", d.Headings, d.Related)
	}
}

func archiveGET(t *testing.T, e *search.Engine) func(string, int, string) string {
	t.Helper()
	tmpl, err := web.LoadTemplates(server.TemplateFuncs())
	check(t, err)
	s, err := server.New(e, server.Options{StaticFS: web.StaticFS(), Templates: tmpl})
	check(t, err)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return func(path string, code int, mime string) string {
		t.Helper()
		r, err := ts.Client().Get(ts.URL + path)
		check(t, err)
		defer r.Body.Close()
		b, err := io.ReadAll(r.Body)
		check(t, err)
		if r.StatusCode != code || !strings.HasPrefix(r.Header.Get("Content-Type"), mime) || len(b) == 0 {
			t.Fatalf("%s: %s %s body=%.120s", path, r.Status, r.Header.Get("Content-Type"), b)
		}
		if r.Header.Get("X-Content-Type-Options") != "nosniff" || r.Header.Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Errorf("%s security headers: %v", path, r.Header)
		}
		return string(b)
	}
}

func TestHTTPServerRoutesAndSecurity(t *testing.T) {
	get := archiveGET(t, fixture(t))
	for _, tc := range []struct{ path, keys string }{
		{"/api/health", "status engine"}, {"/api/stats", "total_documents total_chunks"}, {"/api/municipalities", "municipalities"}, {"/api/search?q=pond", "hits facets detected_entities"}, {"/api/ask?q=pond", "brief hits"}, {"/api/suggest?q=bhang", "suggestions"}, {"/api/documents", "documents"}, {"/api/documents/manesar/manesar-pond", "id title headings related html_content"}, {"/api/wards/manesar", "wards municipality"}, {"/api/officers/manesar", "officers municipality"},
	} {
		raw := get(tc.path, 200, "application/json")
		var obj map[string]json.RawMessage
		check(t, json.Unmarshal([]byte(raw), &obj))
		for _, k := range strings.Fields(tc.keys) {
			if v := obj[k]; len(v) == 0 || string(v) == "null" || string(v) == "[]" {
				t.Errorf("%s missing %s: %s", tc.path, k, raw)
			}
		}
	}
	var missing struct{ Error string }
	check(t, json.Unmarshal([]byte(get("/api/documents/manesar/missing", 404, "application/json")), &missing))
	if missing.Error == "" {
		t.Error("missing document error message")
	}
	for _, tc := range []struct{ path, want string }{{"/", "Search the civic record"}, {"/search?q=ward+9", "Bhangrola pond tender"}, {"/doc/manesar/manesar-pond", "must arrive within 14 days"}, {"/wards", "Asha Fixture"}, {"/index", "Bhangrola pond tender"}, {"/search?q=" + url.QueryEscape(scriptTag), "&lt;script&gt;"}} {
		html := get(tc.path, 200, "text/html")
		if !strings.Contains(html, tc.want) || strings.Contains(html, scriptTag) {
			t.Errorf("%s: wanted %q in safely rendered page", tc.path, tc.want)
		}
	}
	get("/static/styles.css", 200, "text/css")
	get("/static/app.js", 200, "")
	for _, m := range []string{"gurugram", "all"} {
		html := get("/?municipality="+m, 200, "text/html")
		class := "city-option active"
		if m == "all" {
			class = "city-option city-pinned active"
		}
		link := regexp.MustCompile(`<a class="` + class + `"[^>]* href="/\?municipality=` + m + `"[^>]* aria-current="true"`)
		if !link.MatchString(html) {
			t.Errorf("%s home missing active municipality link", m)
		}
		for _, want := range []string{`name="municipality" value="` + m + `"`, `href="/?municipality=manesar"`} {
			if !strings.Contains(html, want) {
				t.Errorf("%s home missing %s", m, want)
			}
		}
	}
	home := get("/?municipality=all", 200, "text/html")
	if !strings.Contains(home, `class="city-group-title">Haryana</p>`) ||
		!strings.Contains(home, `data-city-terms="gurugram  gurugram Haryana Gurugram"`) ||
		strings.Contains(home, `>Urban Local Bodies, Haryana</a>`) {
		t.Error("aggregate home should group by state, support district search, and attribute sources on records")
	}
	home = get("/?municipality=gurugram", 200, "text/html")
	if !strings.Contains(home, `class="city-name">Gurugram</span>`) || !strings.Contains(home, "Search the Gurugram civic archive") {
		t.Error("city without a short name should have a visible, usable label")
	}
	if html := get("/wards?municipality=manesar", 200, "text/html"); !strings.Contains(html, "Asha Fixture") ||
		!strings.Contains(html, `href="https://example.org/wards.pdf#page=9"`) ||
		!strings.Contains(html, "22 Nov 2026") {
		t.Error("scoped directory missing Manesar engineer or ward source")
	}
	contacts := get("/contacts?municipality=manesar", 200, "text/html")
	for _, want := range []string{`href="tel:01234567890"`, `href="mailto:contact@example.org"`, `href="https://example.org/contact"`, "Public duty desk"} {
		if !strings.Contains(contacts, want) {
			t.Errorf("public contacts page missing %s", want)
		}
	}
	if html := get("/contacts?municipality=gurugram", 200, "text/html"); strings.Contains(html, "Public duty desk") ||
		!strings.Contains(html, "No public contacts have been verified") {
		t.Error("empty contact directory shows the wrong city or no empty state")
	}
	if html := get("/contacts", 200, "text/html"); !strings.Contains(html, `href="/contacts?municipality=manesar"`) ||
		strings.Contains(html, `href="tel:01234567890"`) {
		t.Error("national contact directory should link to city pages")
	}
	var municipalities struct{ Municipalities []model.MunicipalityProfile }
	check(t, json.Unmarshal([]byte(get("/api/municipalities", 200, "application/json")), &municipalities))
	if !slices.ContainsFunc(municipalities.Municipalities, func(p model.MunicipalityProfile) bool {
		return p.Slug == "manesar" && len(p.PublicContacts) == 1 && p.PublicContacts[0].Email == "contact@example.org"
	}) {
		t.Error("public contacts missing from municipalities API")
	}
	html := get("/search?q=pond&municipality=all", 200, "text/html")
	for _, m := range []string{"manesar", "gurugram"} {
		if !strings.Contains(html, `href="/doc/`+m+`/`+m+`-pond"`) {
			t.Errorf("all-city results missing %s pond document", m)
		}
	}
	var byName, byRole model.AskResponse
	check(t, json.Unmarshal([]byte(get("/api/ask?municipality=all&mode=keyword&q=Leela+Dummyfield", 200, "application/json")), &byName))
	check(t, json.Unmarshal([]byte(get("/api/ask?municipality=all&mode=keyword&q=junior+engineer", 200, "application/json")), &byRole))
	if byName.Total != 0 || !slices.ContainsFunc(byName.Brief.Citations, func(c model.PromptedCitation) bool {
		return c.Municipality == "manesar" && strings.Contains(c.Title, "Leela Dummyfield")
	}) || !slices.ContainsFunc(byName.Brief.Citations, func(c model.PromptedCitation) bool {
		return c.Municipality == "gurugram" && strings.Contains(c.Title, "Leela Dummyfield")
	}) {
		t.Errorf("named official should remain searchable without a matching document: %+v", byName.Brief)
	}
	if byRole.Total != 0 || len(byRole.Brief.Citations) != 0 {
		t.Errorf("a role alone should not pick an arbitrary city: %+v", byRole.Brief)
	}
}

func TestDateSortingSearchAndIndex(t *testing.T) {
	root, db := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "manesar")
	check(t, os.Mkdir(dir, 0700))
	write := func(name, body string) {
		t.Helper()
		check(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0600))
	}
	write("municipality.json", `{"slug":"manesar","name":"Manesar","shortName":"Manesar"}`)
	for _, d := range []struct{ id, title, date, deadline, extra string }{
		{"recent", "Cedar notice", "2026-11-18", "14 days", "\n\n| Ward | Work |\n| --- | --- |\n| 9 | Drain repair |"},
		{"middle", "Ash notice", "2024-03-10", "21 days", " Bhangrola is the work site."},
		{"early", "Elm notice", "2021-01-05", "28 days", ""},
		{"unknown", "Birch notice", "", "35 days", ""},
		{"year", "Oak notice", "2025", "42 days", ""},
		{"month", "Pine notice", "2025-09", "49 days", ""},
	} {
		write(d.id+".md", fmt.Sprintf("---\nid: %s\ntitle: %s\nmunicipality: manesar\ncategory: tenders-works\ndoc_type: notice\ndate: %q\nsource_url: https://example.org/%s\n---\nThe **pond** repair tender requires bids within %s.%s", d.id, d.title, d.date, d.id, d.deadline, d.extra))
	}
	emb := mockEmbedder{}
	_, err := indexer.BuildIndex(t.Context(), root, db, emb)
	check(t, err)
	get := archiveGET(t, openEngine(t, db, emb))
	readJSON := func(path string, dst any) {
		t.Helper()
		check(t, json.Unmarshal([]byte(get(path, 200, "application/json")), dst))
	}
	desc := []string{"recent", "middle", "early", "year", "month", "unknown"}
	asc := []string{"early", "middle", "recent", "year", "month", "unknown"}
	alpha := []string{"middle", "unknown", "recent", "early", "year", "month"}
	hitIDs := func(hits []model.SearchHit) []string {
		var ids []string
		for _, h := range hits {
			ids = append(ids, h.DocID)
		}
		return ids
	}
	for _, tc := range []struct {
		sort string
		want []string
	}{{"date_desc", desc}, {"date_asc", asc}, {"title", alpha}} {
		var index struct{ Documents []model.ArchiveDocument }
		readJSON("/api/documents?sort="+tc.sort, &index)
		var ids []string
		for _, d := range index.Documents {
			ids = append(ids, d.ID)
		}
		if !slices.Equal(ids, tc.want) {
			t.Errorf("documents %s: got %v, want %v", tc.sort, ids, tc.want)
		}
		if tc.sort != "title" {
			var result model.SearchResponse
			readJSON("/api/search?q=pond&sort="+tc.sort, &result)
			ids := hitIDs(result.Hits)
			if len(ids) == len(tc.want) {
				slices.Sort(ids[3:5])
			}
			want := slices.Clone(tc.want)
			slices.Sort(want[3:5])
			if !slices.Equal(ids, want) {
				t.Errorf("search %s: got %v, want %v", tc.sort, ids, tc.want)
			}
			var weak model.SearchResponse
			readJSON("/api/search?q=xzqvplmxxxx&mode=vector&sort="+tc.sort, &weak)
			if weak.Total != 0 || len(weak.Hits) != 0 {
				t.Errorf("vector %s returned irrelevant hits: %v", tc.sort, hitIDs(weak.Hits))
			}
		}
	}
	var relevance model.SearchResponse
	readJSON("/api/search?q=pond&sort=relevance", &relevance)
	if relevance.Sort != "relevance" || len(relevance.Hits) != len(desc) {
		t.Errorf("relevance: sort=%q hits=%v", relevance.Sort, hitIDs(relevance.Hits))
	}
	for _, path := range []string{"/api/search?q=pond&sort=title", "/api/search?q=pond&sort=bogus", "/api/documents?sort=relevance"} {
		get(path, 400, "application/json")
	}
	for _, path := range []string{"/search?sort=title", "/search?q=pond&sort=title", "/index?sort=relevance"} {
		get(path, 400, "text/plain")
	}
	for _, order := range []string{"date_desc", "date_asc"} {
		page := get("/index?q=Ced+works&sort="+order, 200, "text/html")
		_, register, found := strings.Cut(page, `<tbody data-index-rows>`)
		body, _, closed := strings.Cut(register, `</tbody>`)
		if !found || !closed || strings.Count(body, "data-row") != 1 || !strings.Contains(body, `data-id="recent"`) {
			t.Errorf("index %s did not filter visible rows: %s", order, body)
		}
	}
	if page := get("/index?q=xzqvplmxxxx", 200, "text/html"); !strings.Contains(page, `data-empty-index>No records match`) {
		t.Error("index missing no-matches state")
	}
	var grounded model.AskResponse
	const evidenceQuery = "/api/ask?q=Bhangrola+pond&mode=keyword&limit=1&sort="
	readJSON(evidenceQuery+"relevance", &grounded)
	if len(grounded.Brief.Citations) == 0 || grounded.Brief.Citations[0].DocID != "middle" {
		t.Fatalf("fixture did not rank the most relevant evidence first: %v", grounded.Brief.Citations)
	}
	for _, tc := range []struct{ sort, id string }{{"date_asc", "early"}, {"date_desc", "recent"}} {
		var ordered model.AskResponse
		readJSON(evidenceQuery+tc.sort, &ordered)
		if !slices.Equal(hitIDs(ordered.Hits), []string{tc.id}) || !reflect.DeepEqual(ordered.Brief, grounded.Brief) {
			t.Errorf("%s changed evidence instead of just display order: hits=%v citations=%v", tc.sort, hitIDs(ordered.Hits), ordered.Brief.Citations)
		}
	}
	var ask model.AskResponse
	readJSON("/api/ask?q=Cedar+pond&mode=keyword&sort=date_desc&limit=1", &ask)
	if ids := hitIDs(ask.Hits); !slices.Equal(ids, []string{"recent"}) {
		t.Errorf("ask date_desc: got %v, want recent", ids)
	} else if snippet := ask.Hits[0].Snippet; !strings.Contains(snippet, "pond") || strings.ContainsAny(snippet, "*|") {
		t.Errorf("snippet contains Markdown or omits source text: %q", snippet)
	}
	if !slices.ContainsFunc(ask.Brief.Facts, func(f model.ExtractedFact) bool {
		if f.Value != "14 days" || f.CitationIndex < 1 || f.CitationIndex > len(ask.Brief.Citations) {
			return false
		}
		c := ask.Brief.Citations[f.CitationIndex-1]
		return c.Index == f.CitationIndex && c.DocID == "recent" && c.SourceURL == "https://example.org/recent" && strings.Contains(c.Excerpt, f.Value)
	}) {
		t.Errorf("missing grounded 14-day deadline: facts=%v citations=%v", ask.Brief.Facts, ask.Brief.Citations)
	}
	const recent = `datetime="2026-11-18">18 Nov 2026</time>`
	const middle = `datetime="2024-03-10">10 Mar 2024</time>`
	const early = `datetime="2021-01-05">05 Jan 2021</time>`
	for _, path := range []string{"/search?q=pond&sort=date_desc", "/search?q=pond&sort=date_asc", "/index?sort=date_asc"} {
		page := get(path, 200, "text/html")
		dates := []string{early, middle, recent}
		sort := "date_asc"
		if strings.HasSuffix(path, "date_desc") {
			dates = []string{recent, middle, early}
			sort = "date_desc"
		}
		if strings.HasPrefix(path, "/index") {
			if !strings.Contains(page, `data-index-sort`) || !strings.Contains(page, `<option value="date_asc" selected>`) {
				t.Error("index missing selected oldest-first sort control")
			}
		} else {
			active := regexp.MustCompile(`<a href="/search\?[^"]*sort=` + sort + `"\s+aria-current="true"`)
			if !active.MatchString(page) {
				t.Errorf("%s: missing active sort link", path)
			}
			_, page, _ = strings.Cut(page, `id="results"`)
		}
		for _, date := range dates {
			var found bool
			_, page, found = strings.Cut(page, date)
			if !found {
				t.Errorf("%s: missing or out-of-order date %s", path, date)
				break
			}
		}
		partial, _, found := strings.Cut(page, "Undated")
		if !found || !strings.Contains(partial, `datetime="2025">2025</time>`) || !strings.Contains(partial, `datetime="2025-09">Sep 2025</time>`) {
			t.Errorf("%s: partial dates should follow full dates and precede undated", path)
		}
	}
	home := get("/", 200, "text/html")
	for _, marker := range []string{"Plus+Jakarta+Sans", "family=Inter", "JetBrains+Mono", "data-theme-toggle", "data-city-filter"} {
		if !strings.Contains(home, marker) {
			t.Errorf("home missing %s", marker)
		}
	}
	css := get("/static/styles.css", 200, "text/css")
	for _, marker := range []string{`[data-theme="dark"]`, "prefers-color-scheme: dark"} {
		if !strings.Contains(css, marker) {
			t.Errorf("stylesheet missing %s", marker)
		}
	}
}

func TestFilteredKeywordAndAllMunicipalitySearch(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	for _, m := range []string{"manesar", "gurugram"} {
		base := filepath.Join(root, m)
		check(t, os.Mkdir(base, 0700))
		write := func(name, s string) {
			t.Helper()
			check(t, os.WriteFile(filepath.Join(base, name), []byte(s), 0600))
		}
		write("municipality.json", fmt.Sprintf(`{"slug":%q,"name":%q}`, m, m))
		doc := func(id, title, category, kind string, ward int, body string) {
			write(id+".md", fmt.Sprintf("---\nid: %s-%s\nmunicipality: %s\ntitle: %s\ncategory: %s\ndoc_type: %s\nwards: [%d]\n---\n%s", m, id, m, title, category, kind, ward, body))
		}
		doc("guide", "Drainage help", "citizen-services", "guide", 9, "For pond permits visit https://ulbcluhry.org/permits.")
		if m == "manesar" {
			// More high-ranking chunks than the 120-candidate keyword window.
			doc("works", "Pond", "tenders-works", "tender", 3, strings.Repeat("## Pond\n"+strings.Repeat("pond ", 32)+"\n", 125))
		}
	}
	emb := mockEmbedder{}
	_, err := indexer.BuildIndex(t.Context(), root, dir, emb)
	check(t, err)
	raw, err := os.ReadFile(filepath.Join(dir, indexer.CurrentFilename))
	check(t, err)
	gen := strings.TrimSpace(string(raw))
	if filepath.Dir(gen) != indexer.GenerationsDir {
		t.Fatalf("invalid CURRENT: %q", gen)
	}
	db, vec, ok := indexer.ResolveDataPaths(dir)
	if !ok || db != filepath.Join(dir, gen, indexer.SQLiteFilename) || vec != filepath.Join(dir, gen, indexer.VectorDirname) {
		t.Fatalf("CURRENT %q resolved to %q, %q, ready=%t", gen, db, vec, ok)
	}
	e := openEngine(t, dir, emb)
	for _, req := range []model.SearchRequest{{}, {Category: "citizen-services"}, {DocType: "guide"}, {Ward: 9}} {
		want := "manesar-guide"
		if req == (model.SearchRequest{}) {
			want = "manesar-works"
		}
		req.Query, req.Municipality, req.Mode, req.Limit = "pond", "manesar", model.SearchModeKeyword, 1
		r, err := e.Search(t.Context(), req)
		check(t, err)
		if len(r.Hits) != 1 || r.Hits[0].DocID != want {
			t.Errorf("%+v: want %s, got %v", req, want, r.Hits)
		}
	}
	a, err := e.Ask(t.Context(), model.SearchRequest{Municipality: "all", Query: "permits", Mode: model.SearchModeKeyword})
	check(t, err)
	for _, m := range []string{"manesar", "gurugram"} {
		if !slices.ContainsFunc(a.Hits, func(h model.SearchHit) bool { return h.Municipality == m && h.DocID == m+"-guide" }) {
			t.Errorf("all-city search missing %s guide", m)
		}
	}
	if !slices.ContainsFunc(a.Brief.ActionLinks, func(l model.ActionLink) bool { return l.URL == "https://ulbcluhry.org/permits" }) {
		t.Errorf("missing permit portal: %v", a.Brief.ActionLinks)
	}
	html, err := server.RenderMarkdownWithLinks("[permit guide](../guides/permit.md#forms)", "gurugram", "records/application.md", map[string]string{"guides/permit.md": "gurugram-guide"})
	check(t, err)
	if !strings.Contains(string(html), `href="/doc/gurugram/gurugram-guide#forms"`) {
		t.Errorf("relative link rendered as %s", html)
	}
	check(t, os.Remove(filepath.Join(dir, gen, indexer.CompleteMarker)))
	if _, _, ok := indexer.ResolveDataPaths(dir); ok {
		t.Error("incomplete generation resolved as ready")
	}
}
