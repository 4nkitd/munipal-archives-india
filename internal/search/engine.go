package search

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/municipal-archives-india/manesar/internal/model"
	chromem "github.com/philippgille/chromem-go"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
	_ "modernc.org/sqlite"
)

type Engine struct {
	db       *sql.DB
	col      *chromem.Collection
	embedder model.Embedder
}

var _ model.SearchService = (*Engine)(nil)

func NewEngine(sqlDB *sql.DB, vectorDB *chromem.DB, embedder model.Embedder) (*Engine, error) {
	if sqlDB == nil {
		return nil, errors.New("search requires an archive database")
	}
	e := &Engine{db: sqlDB, embedder: embedder}
	if vectorDB != nil {
		e.col = vectorDB.GetCollection("archive_chunks", nil)
	}
	return e, nil
}
func municipality(m string) string { return cmp.Or(strings.TrimSpace(m), "manesar") }
func scope(m string) string {
	if m == "all" {
		return ""
	}
	return municipality(m)
}
func records[T any](ctx context.Context, db *sql.DB, q string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var b []byte
		var v T
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (e *Engine) ListMunicipalities(ctx context.Context) ([]model.MunicipalityProfile, error) {
	return records[model.MunicipalityProfile](ctx, e.db, "SELECT profile_json FROM municipalities ORDER BY slug")
}
func (e *Engine) ListWards(ctx context.Context, m string) ([]model.WardRecord, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT municipality,record_json FROM wards WHERE (?='' OR municipality=?) ORDER BY municipality,ward_number", scope(m), scope(m))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WardRecord{}
	for rows.Next() {
		var muni string
		var b []byte
		var w model.WardRecord
		if err := rows.Scan(&muni, &b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, err
		}
		w.Municipality = muni
		out = append(out, w)
	}
	return out, rows.Err()
}
func (e *Engine) ListOfficers(ctx context.Context, m string) ([]model.OfficerRecord, error) {
	rows, err := e.db.QueryContext(ctx, "SELECT municipality,record_json FROM officers WHERE (?='' OR municipality=?) ORDER BY municipality,sr_no", scope(m), scope(m))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.OfficerRecord{}
	for rows.Next() {
		var muni string
		var b []byte
		var o model.OfficerRecord
		if err := rows.Scan(&muni, &b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &o); err != nil {
			return nil, err
		}
		o.Municipality = muni
		out = append(out, o)
	}
	return out, rows.Err()
}

const columns = `d.id,d.municipality,d.title,coalesce(d.category,''),coalesce(d.subcategory,''),coalesce(d.doc_type,''),coalesce(d.date,''),coalesce(d.source_url,''),coalesce(d.source_portal,''),coalesce(d.original_filename,''),coalesce(d.department,''),d.wards_json,d.sectors_json,d.villages_json,d.tags_json,d.contacts_json,coalesce(d.action_url,''),coalesce(d.summary,''),coalesce(d.verified_at,''),coalesce(d.file_path,''),coalesce(d.word_count,0),coalesce(d.chunk_count,0)`

type scanner interface{ Scan(...any) error }

func scanDoc(row scanner, d *model.ArchiveDocument, extra ...any) error {
	var raw [5][]byte
	args := []any{&d.ID, &d.Municipality, &d.Title, &d.Category, &d.Subcategory, &d.DocType, &d.Date, &d.SourceURL, &d.SourcePortal, &d.OriginalFilename, &d.Department, &raw[0], &raw[1], &raw[2], &raw[3], &raw[4], &d.ActionURL, &d.Summary, &d.VerifiedAt, &d.FilePath, &d.WordCount, &d.ChunkCount}
	if err := row.Scan(append(args, extra...)...); err != nil {
		return err
	}
	for i, dst := range []any{&d.Wards, &d.Sectors, &d.Villages, &d.Tags, &d.Contacts} {
		if len(raw[i]) > 0 {
			if err := json.Unmarshal(raw[i], dst); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *Engine) ListDocuments(ctx context.Context, m, cat, dt string, w int) ([]model.ArchiveDocument, error) {
	return e.ListDocumentsSorted(ctx, m, cat, dt, w, model.SortDateDesc)
}

func compareDates(a, b string, ascending bool) int {
	ad, bd := model.PublicationDay(a), model.PublicationDay(b)
	switch {
	case ad == "" && bd == "":
		return cmp.Compare(blankDate(a), blankDate(b))
	case ad == "":
		return 1
	case bd == "":
		return -1
	case ascending:
		return cmp.Compare(ad, bd)
	default:
		return cmp.Compare(bd, ad)
	}
}

func blankDate(date string) int {
	if strings.TrimSpace(date) == "" {
		return 1
	}
	return 0
}

func (e *Engine) ListDocumentsSorted(ctx context.Context, m, cat, dt string, w int, sortBy string) ([]model.ArchiveDocument, error) {
	if sortBy != "" && sortBy != model.SortDateDesc && sortBy != model.SortDateAsc && sortBy != model.SortTitle {
		return nil, errors.New("sort must be date_desc, date_asc or title")
	}
	rows, err := e.db.QueryContext(ctx, `SELECT `+columns+` FROM documents d WHERE (?='' OR municipality=?) AND (?='' OR category=?) AND (?='' OR doc_type=?) AND (?=0 OR EXISTS(SELECT 1 FROM json_each(d.wards_json) WHERE value=?))`, scope(m), scope(m), cat, cat, dt, dt, w, w)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ArchiveDocument{}
	for rows.Next() {
		var d model.ArchiveDocument
		if err := scanDoc(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b model.ArchiveDocument) int {
		if sortBy != model.SortTitle {
			if date := compareDates(a.Date, b.Date, sortBy == model.SortDateAsc); date != 0 {
				return date
			}
		}
		return cmp.Or(cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)), cmp.Compare(a.Municipality, b.Municipality), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}
func (e *Engine) vectors(ctx context.Context, v []float32, m string, filtered bool) ([]chromem.Result, error) {
	if e.col == nil || len(v) == 0 {
		return nil, nil
	}
	total := e.col.Count()
	if total == 0 {
		return nil, nil
	}
	n := min(60, total)
	if filtered {
		n = total
	}
	var where map[string]string
	if m != "" {
		where = map[string]string{"municipality": m}
	}
	rs, err := e.col.QueryEmbedding(ctx, v, n, where, nil)
	if err != nil && where != nil && (strings.Contains(strings.ToLower(err.Error()), "nresults") || strings.Contains(strings.ToLower(err.Error()), "fewer documents")) {
		if n = min(n, e.col.Count()); n == 0 {
			return nil, nil
		}
		rs, err = e.col.QueryEmbedding(ctx, v, n, nil, nil)
	}
	return slices.DeleteFunc(rs, func(r chromem.Result) bool { return m != "" && r.Metadata["municipality"] != m }), err
}

var tagPattern = regexp.MustCompile(`<[^>\n]*>`)

func plain(s string) string {
	return strings.Join(strings.Fields(tagPattern.ReplaceAllString(s, " ")), " ")
}
func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	t := string(r[:n])
	if i := strings.LastIndexByte(t, ' '); i > len(t)/2 {
		t = t[:i]
	}
	return t + "…"
}

func searchSnippet(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if !mdTableSep.MatchString(line) {
			if text := cleanMarkdownLine(line); text != "" {
				lines = append(lines, text)
			}
		}
	}
	return shorten(strings.Join(lines, " "), 320)
}

func terms(q string) []string {
	stop := " a an are as at and by can do does for from how i in is it me my of on or the to what when where which who with not near "
	all := []string{}
	filtered := []string{}
	for _, s := range words.FindAllString(norm.NFC.String(strings.ToLower(q)), 64) {
		all = unique(all, s)
		if !strings.Contains(stop, " "+s+" ") {
			filtered = unique(filtered, s)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return all
}

type rank struct {
	v, k    int
	kwHit   bool
	sim, bm float64
	snippet string
}

func (e *Engine) keywords(ctx context.Context, q, m, cat, dt string, ward, limit int, ranks map[string]*rank) error {
	ts := terms(q)
	if len(ts) == 0 {
		return nil
	}
	for i, t := range ts {
		ts[i] = `"` + t + `"`
	}
	pos := 0
	for _, join := range []string{" AND ", " OR "} {
		rows, err := e.db.QueryContext(ctx, `SELECT f.chunk_id,-bm25(chunks_fts,0,0,0,4,2.5,1,2),snippet(chunks_fts,5,'','',' … ',36) FROM chunks_fts f JOIN documents d ON d.id=f.doc_id AND d.municipality=f.municipality WHERE chunks_fts MATCH ? AND (?='' OR f.municipality=?) AND (?='' OR d.category=?) AND (?='' OR d.doc_type=?) AND (?=0 OR EXISTS(SELECT 1 FROM json_each(d.wards_json) WHERE value=?)) ORDER BY 2 DESC,f.chunk_id LIMIT ?`, strings.Join(ts, join), m, m, cat, cat, dt, dt, ward, ward, limit)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, s string
			var score float64
			if err := rows.Scan(&id, &score, &s); err != nil {
				rows.Close()
				return err
			}
			r := ranks[id]
			if r == nil {
				r = &rank{}
				ranks[id] = r
			}
			if r.k == 0 {
				pos++
				r.k = pos
				r.bm = score
				r.snippet = s
			}
			r.kwHit = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(ts) == 1 {
			break
		}
	}
	return nil
}
func (e *Engine) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error) {
	res, _, err := e.searchWithEvidence(ctx, req)
	return res, err
}

func (e *Engine) searchWithEvidence(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, []model.SearchHit, error) {
	start := time.Now()
	req.Municipality = municipality(req.Municipality)
	m := scope(req.Municipality)
	req.Mode = cmp.Or(req.Mode, model.SearchModeHybrid)
	req.Sort = cmp.Or(strings.TrimSpace(req.Sort), model.SortRelevance)
	byDate := req.Sort == model.SortDateDesc || req.Sort == model.SortDateAsc
	if !byDate && req.Sort != model.SortRelevance {
		return nil, nil, errors.New("sort must be relevance, date_desc or date_asc")
	}
	if req.Limit == 0 {
		req.Limit = 12
	}
	req.Limit = max(1, min(model.MaxSearchLimit, req.Limit))
	ws, err := e.ListWards(ctx, req.Municipality)
	if err != nil {
		return nil, nil, err
	}
	out := &model.SearchResponse{Query: req.Query, Municipality: req.Municipality, Mode: req.Mode, Sort: req.Sort, DetectedEntities: DetectEntities(req.Query, ws), Hits: []model.SearchHit{}}
	hasFilter := req.Category != "" || req.DocType != "" || req.Ward > 0
	ranks := map[string]*rank{}
	if req.Mode != model.SearchModeKeyword && e.embedder != nil && e.col != nil && e.col.Count() > 0 && strings.TrimSpace(req.Query) != "" {
		v, err := e.embedder.EmbedQuery(ctx, req.Query)
		if err == nil {
			var rs []chromem.Result
			rs, err = e.vectors(ctx, v, m, hasFilter || byDate)
			for i, r := range rs {
				ranks[r.ID] = &rank{v: i + 1, sim: float64(r.Similarity)}
			}
		}
		if err != nil && (req.Mode == model.SearchModeVector || ctx.Err() != nil) {
			return nil, nil, err
		}
	}
	if req.Mode != model.SearchModeVector {
		limit := 120
		if byDate {
			limit = -1
		}
		if hasFilter {
			// Rank eligible filtered chunks in their own clean 1..N sequence first,
			// then preserve keyword eligibility on unfiltered chunks for facet counts.
			if err := e.keywords(ctx, req.Query, m, req.Category, req.DocType, req.Ward, limit, ranks); err != nil {
				return nil, nil, err
			}
			unfiltered := map[string]*rank{}
			if err := e.keywords(ctx, req.Query, m, "", "", 0, limit, unfiltered); err != nil {
				return nil, nil, err
			}
			for id, ur := range unfiltered {
				if existing, exists := ranks[id]; exists {
					existing.kwHit = true
				} else {
					ranks[id] = ur
				}
			}
		} else {
			if err := e.keywords(ctx, req.Query, m, "", "", 0, limit, ranks); err != nil {
				return nil, nil, err
			}
		}
	}
	matchesFilters := func(h model.SearchHit) bool {
		return (req.Category == "" || h.Category == req.Category) && (req.DocType == "" || h.DocType == req.DocType) && (req.Ward == 0 || slices.Contains(h.Wards, req.Ward))
	}
	best := map[[2]string]model.SearchHit{}
	ids := make([]string, 0, len(ranks))
	for id, rank := range ranks {
		if !byDate || rank.kwHit || rank.sim >= 0.48 {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		encodedIDs, err := json.Marshal(ids)
		if err != nil {
			return nil, nil, err
		}
		rows, err := e.db.QueryContext(ctx, `SELECT `+columns+`,c.chunk_id,coalesce(c.heading,''),c.body FROM chunks c JOIN documents d ON d.id=c.doc_id AND d.municipality=c.municipality WHERE (?='' OR d.municipality=?) AND c.chunk_id IN (SELECT value FROM json_each(?))`, m, m, string(encodedIDs))
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var d model.ArchiveDocument
			var id, heading, body string
			if err := scanDoc(rows, &d, &id, &heading, &body); err != nil {
				return nil, nil, err
			}
			r := ranks[id]
			h := model.SearchHit{DocID: d.ID, ChunkID: id, Municipality: d.Municipality, Title: d.Title, Category: d.Category, Subcategory: d.Subcategory, DocType: d.DocType, Date: d.Date, Department: d.Department, SourceURL: d.SourceURL, SourcePortal: d.SourcePortal, ActionURL: d.ActionURL, Wards: d.Wards, Sectors: d.Sectors, Villages: d.Villages, Tags: d.Tags, Contacts: d.Contacts, Summary: d.Summary, Heading: heading, MatchedChunkText: body, Snippet: searchSnippet(cmp.Or(r.snippet, body)), VectorScore: r.sim, KeywordScore: r.bm}
			if r.v > 0 {
				h.Score += 1 / float64(45+r.v)
				h.MatchSources = append(h.MatchSources, "vector")
			}
			if r.k > 0 {
				h.Score += 1 / float64(45+r.k)
				h.MatchSources = append(h.MatchSources, "keyword")
			}
			ent := out.DetectedEntities
			if slices.ContainsFunc(h.Wards, func(w int) bool { return slices.Contains(ent.Wards, w) }) {
				h.Score += 0.005
			}
			if slices.ContainsFunc(h.Sectors, func(s string) bool { return slices.Contains(ent.Sectors, sector(s)) }) {
				h.Score += 0.006
			}
			if slices.ContainsFunc(h.Villages, func(s string) bool {
				return slices.ContainsFunc(ent.Villages, func(v string) bool { return strings.EqualFold(s, v) })
			}) {
				h.Score += 0.006
			}
			key := [2]string{d.Municipality, d.ID}
			old, ok := best[key]
			if !ok || h.Score > old.Score || h.Score == old.Score && id < old.ChunkID {
				best[key] = h
			}
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	cats, types, wcounts := map[string]int{}, map[string]int{}, map[int]int{}
	for _, h := range best {
		cats[h.Category]++
		types[h.DocType]++
		for _, w := range h.Wards {
			wcounts[w]++
		}
		if matchesFilters(h) {
			out.Hits = append(out.Hits, h)
		}
	}
	buckets := func(m map[string]int) []model.FacetBucket {
		b := []model.FacetBucket{}
		for v, n := range m {
			b = append(b, model.FacetBucket{Value: v, Count: n})
		}
		slices.SortFunc(b, func(a, b model.FacetBucket) int {
			return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Value, b.Value))
		})
		return b
	}
	out.Facets.Categories = buckets(cats)
	out.Facets.DocTypes = buckets(types)
	for v, n := range wcounts {
		out.Facets.Wards = append(out.Facets.Wards, model.WardFacetBucket{Value: v, Count: n})
	}
	slices.SortFunc(out.Facets.Wards, func(a, b model.WardFacetBucket) int { return cmp.Compare(a.Value, b.Value) })

	// Always rank by relevance first to capture stable evidence for Prompted Brief synthesis.
	byRelevance := func(a, b model.SearchHit) int {
		score := cmp.Compare(b.Score, a.Score)
		ad, bd := strings.TrimSpace(a.Date), strings.TrimSpace(b.Date)
		date := cmp.Compare(bd, ad)
		tie := cmp.Or(cmp.Compare(a.Municipality, b.Municipality), cmp.Compare(a.DocID, b.DocID))
		return cmp.Or(score, date, tie)
	}
	slices.SortFunc(out.Hits, byRelevance)
	evidence := append([]model.SearchHit(nil), out.Hits[:min(4, len(out.Hits))]...)

	if byDate {
		slices.SortFunc(out.Hits, func(a, b model.SearchHit) int {
			score := cmp.Compare(b.Score, a.Score)
			tie := cmp.Or(cmp.Compare(a.Municipality, b.Municipality), cmp.Compare(a.DocID, b.DocID))
			date := compareDates(a.Date, b.Date, req.Sort == model.SortDateAsc)
			return cmp.Or(date, score, tie)
		})
	}
	out.Total = len(out.Hits)
	out.Hits = out.Hits[:min(req.Limit, out.Total)]
	out.TookMs = time.Since(start).Milliseconds()
	return out, evidence, nil
}

func (e *Engine) GetDocument(ctx context.Context, m, id string) (*model.ArchiveDocumentFull, error) {
	d := &model.ArchiveDocumentFull{}
	err := scanDoc(e.db.QueryRowContext(ctx, `SELECT `+columns+`,coalesce(d.content,'') FROM documents d WHERE id=? AND (?='' OR municipality=?) ORDER BY municipality LIMIT 1`, id, scope(m), scope(m)), &d.ArchiveDocument, &d.Content)
	if err != nil {
		return nil, err
	}
	source := []byte(d.Content)
	root := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID())).Parser().Parse(text.NewReader(source))
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); entering && ok && (h.Level == 2 || h.Level == 3) {
			id, _ := h.AttributeString("id")
			slug, _ := id.([]byte)
			d.Headings = append(d.Headings, model.HeadingItem{Level: h.Level, Text: string(h.Text(source)), ID: string(slug)})
		}
		return ast.WalkContinue, nil
	})
	if e.col == nil || e.col.Count() == 0 {
		return d, nil
	}
	var first string
	var vec []float32
	err = e.db.QueryRowContext(ctx, `SELECT chunk_id FROM chunks WHERE municipality=? AND doc_id=? ORDER BY chunk_index LIMIT 1`, d.Municipality, id).Scan(&first)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if first != "" {
		if v, err := e.col.GetByID(ctx, first); err == nil {
			vec = v.Embedding
		}
	}
	if len(vec) == 0 && e.embedder != nil {
		vec, err = e.embedder.EmbedQuery(ctx, d.Title+"\n"+d.Summary)
		if err != nil {
			return nil, err
		}
	}
	rs, err := e.vectors(ctx, vec, d.Municipality, false)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{id: true}
	for _, r := range rs {
		rid := r.Metadata["doc_id"]
		if ids[rid] || rid == "" {
			continue
		}
		ids[rid] = true
		x := model.RelatedDocument{ID: rid, Municipality: d.Municipality, Similarity: float64(r.Similarity)}
		err := e.db.QueryRowContext(ctx, `SELECT title,coalesce(category,''),coalesce(date,''),coalesce(summary,'') FROM documents WHERE municipality=? AND id=?`, d.Municipality, rid).Scan(&x.Title, &x.Category, &x.Date, &x.Summary)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d.Related = append(d.Related, x)
		if len(d.Related) == 5 {
			break
		}
	}
	return d, nil
}
func (e *Engine) Stats(ctx context.Context) (*model.IndexStats, error) {
	s := &model.IndexStats{VectorDBEngine: "chromem-go", FTSEngine: "SQLite FTS5"}
	err := e.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM municipalities),count(*),(SELECT count(*) FROM chunks),coalesce(sum(word_count),0),(SELECT count(*) FROM wards),(SELECT count(*) FROM officers) FROM documents`).Scan(&s.MunicipalitiesIndexed, &s.TotalDocuments, &s.TotalChunks, &s.TotalWords, &s.WardsMapped, &s.OfficersMapped)
	if err != nil {
		return nil, err
	}
	rows, err := e.db.QueryContext(ctx, "SELECT key,value FROM index_meta")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return nil, err
		}
		switch k {
		case "embedding_model":
			s.EmbeddingModel = v
		case "embedding_dimensions":
			s.EmbeddingDimensions, _ = strconv.Atoi(v)
		case "indexed_at":
			s.IndexedAt = v
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if e.embedder != nil {
		s.EmbeddingModel = cmp.Or(s.EmbeddingModel, e.embedder.ModelName())
		s.EmbeddingDimensions = cmp.Or(s.EmbeddingDimensions, e.embedder.Dimensions())
	}
	rows, err = e.db.QueryContext(ctx, "SELECT category,count(*) FROM documents GROUP BY category ORDER BY count(*) DESC,category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c model.CategoryCount
		if err := rows.Scan(&c.Category, &c.Count); err != nil {
			return nil, err
		}
		s.Categories = append(s.Categories, c)
	}
	return s, rows.Err()
}
func (e *Engine) Ask(ctx context.Context, req model.SearchRequest) (*model.AskResponse, error) {
	r, evidence, err := e.searchWithEvidence(ctx, req)
	if err != nil {
		return nil, err
	}
	m := r.Municipality
	if m == "all" && len(evidence) > 0 {
		m = evidence[0].Municipality
	}
	w, err := e.ListWards(ctx, m)
	if err != nil {
		return nil, err
	}
	o, err := e.ListOfficers(ctx, m)
	if err != nil {
		return nil, err
	}
	return &model.AskResponse{SearchResponse: *r, Brief: BuildPromptedAnswerForMunicipality(req.Query, r.Municipality, evidence, DetectEntities(req.Query, w), w, o)}, nil
}
