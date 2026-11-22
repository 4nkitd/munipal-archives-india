package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/municipal-archives-india/manesar/internal/model"
	chromem "github.com/philippgille/chromem-go"
	"golang.org/x/text/unicode/norm"
)

const CollectionName = "archive_chunks"

type builder struct {
	ctx        context.Context
	tx         *sql.Tx
	embedder   model.Embedder
	col        *chromem.Collection
	stats      model.IndexStats
	categories map[string]int
	texts      []string
	vectors    []chromem.Document
}

func BuildIndex(ctx context.Context, archivesDir, dataDir string, embedder model.Embedder) (*model.IndexStats, error) {
	if embedder == nil {
		return nil, errors.New("an embedder is required")
	}
	profiles, err := ScanMunicipalities(archivesDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(dataDir, GenerationsDir), 0755); err != nil {
		return nil, err
	}
	genName := fmt.Sprintf("gen-%d", time.Now().UTC().UnixNano())
	genRel := filepath.Join(GenerationsDir, genName)
	genDir := filepath.Join(dataDir, genRel)
	if err = os.MkdirAll(genDir, 0755); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(genDir)
		}
	}()
	dbPath := filepath.Join(genDir, SQLiteFilename)
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	defer tx.Rollback()
	vecDir := filepath.Join(genDir, VectorDirname)
	vdb, err := OpenVectorDB(vecDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	col, err := vdb.CreateCollection(CollectionName, nil, embedder.EmbedQuery)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	b := builder{ctx: ctx, tx: tx, embedder: embedder, col: col, categories: map[string]int{}}
	s := &b.stats
	s.EmbeddingModel = embedder.ModelName()
	s.EmbeddingDimensions = embedder.Dimensions()
	s.VectorDBEngine = "chromem-go (persistent cosine)"
	s.FTSEngine = "SQLite FTS5 (BM25 + porter)"
	for _, p := range profiles {
		if err = b.municipality(archivesDir, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("%s: %w", p.Slug, err)
		}
		s.MunicipalitiesIndexed++
	}
	if err = b.flush(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.IndexedAt = time.Now().UTC().Format(time.RFC3339)
	s.Categories = []model.CategoryCount{}
	for cat, n := range b.categories {
		s.Categories = append(s.Categories, model.CategoryCount{Category: cat, Count: n})
	}
	sort.Slice(s.Categories, func(i, j int) bool { return s.Categories[i].Category < s.Categories[j].Category })
	for k, v := range map[string]string{"embedding_model": s.EmbeddingModel, "embedding_dimensions": strconv.Itoa(s.EmbeddingDimensions), "vector_db_engine": s.VectorDBEngine, "fts_engine": s.FTSEngine, "fts_tokenizer": ftsTokenizerVersion, "indexed_at": s.IndexedAt} {
		if err = b.exec(`INSERT INTO index_meta VALUES(?,?)`, k, v); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Checkpoint WAL into the main SQLite file before publishing the immutable generation.
	_, _ = db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`)
	if err = db.Close(); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(genDir, CompleteMarker), []byte(s.IndexedAt+"\n"), 0644); err != nil {
		return nil, err
	}
	// Publish the new paired SQLite + Vector generation atomically via temp file + rename.
	tmpPtr, err := os.CreateTemp(dataDir, ".CURRENT-")
	if err != nil {
		return nil, err
	}
	tmpPtrName := tmpPtr.Name()
	if _, err = tmpPtr.WriteString(filepath.ToSlash(genRel) + "\n"); err != nil {
		_ = tmpPtr.Close()
		_ = os.Remove(tmpPtrName)
		return nil, err
	}
	if err = tmpPtr.Sync(); err != nil {
		_ = tmpPtr.Close()
		_ = os.Remove(tmpPtrName)
		return nil, err
	}
	if err = tmpPtr.Close(); err != nil {
		_ = os.Remove(tmpPtrName)
		return nil, err
	}
	if err = os.Chmod(tmpPtrName, 0644); err != nil {
		_ = os.Remove(tmpPtrName)
		return nil, err
	}
	if err = os.Rename(tmpPtrName, filepath.Join(dataDir, CurrentFilename)); err != nil {
		_ = os.Remove(tmpPtrName)
		return nil, err
	}
	committed = true
	pruneOldGenerations(filepath.Join(dataDir, GenerationsDir), genName)
	return s, nil
}

func pruneOldGenerations(gensDir, keep string) {
	entries, err := os.ReadDir(gensDir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "gen-") && e.Name() != keep && fileExists(filepath.Join(gensDir, e.Name(), CompleteMarker)) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	// Retain the latest previous complete generation; never delete in-progress builds.
	for i := 0; i < len(names)-1; i++ {
		_ = os.RemoveAll(filepath.Join(gensDir, names[i]))
	}
}

func (b *builder) exec(query string, args ...any) error {
	_, err := b.tx.ExecContext(b.ctx, query, args...)
	return err
}

func (b *builder) municipality(root string, p model.MunicipalityProfile) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err = b.exec(`INSERT INTO municipalities VALUES(?,?)`, p.Slug, string(raw)); err != nil {
		return err
	}
	dir := filepath.Join(root, p.Slug)
	ws, err := LoadMunicipalityWards(dir)
	if err != nil {
		return err
	}
	for _, w := range ws {
		raw, err = json.Marshal(w)
		if err != nil {
			return err
		}
		if err = b.exec(`INSERT INTO wards VALUES(?,?,?)`, p.Slug, w.WardNumber, string(raw)); err != nil {
			return err
		}
	}
	b.stats.WardsMapped += len(ws)
	oset, err := LoadMunicipalityOfficers(dir)
	if err != nil {
		return err
	}
	for _, o := range oset {
		raw, err = json.Marshal(o)
		if err != nil {
			return err
		}
		if err = b.exec(`INSERT INTO officers VALUES(?,?,?)`, p.Slug, o.SrNo, string(raw)); err != nil {
			return err
		}
	}
	b.stats.OfficersMapped += len(oset)
	return filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = b.ctx.Err(); err != nil {
			return err
		}
		if !e.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		d, err := ParseMarkdownFile(path, root)
		if err != nil {
			return err
		}
		if d.Municipality != p.Slug {
			return fmt.Errorf("%s: frontmatter municipality %q differs from directory", path, d.Municipality)
		}
		if err = b.document(d); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
}

func (b *builder) document(d *model.ArchiveDocument) error {
	chunks := ChunkDocument(d)
	d.ChunkCount = len(chunks)
	fields := []any{d.Wards, d.Sectors, d.Villages, d.Tags, d.Contacts}
	for i, v := range fields {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		fields[i] = string(raw)
	}
	if err := b.exec(`INSERT INTO documents VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.Municipality, d.Title, d.Category, d.Subcategory, d.DocType, d.Date, d.SourceURL, d.SourcePortal, d.OriginalFilename, d.Department,
		fields[0], fields[1], fields[2], fields[3], fields[4], d.ActionURL, d.Summary, d.VerifiedAt, d.FilePath, d.WordCount, d.ChunkCount, d.Content); err != nil {
		return err
	}
	entities := []string{geography(d), strings.Join(d.Tags, " "), d.Department, d.DocType}
	for _, c := range d.Contacts {
		entities = append(entities, c.Name, c.Role, c.Phone, c.Email)
	}
	et := norm.NFC.String(strings.Join(entities, " "))
	title := norm.NFC.String(d.Title)
	for _, c := range chunks {
		if err := b.exec(`INSERT INTO chunks VALUES(?,?,?,?,?,?,?)`, c.ChunkID, d.ID, d.Municipality, c.ChunkIndex, c.Heading, c.Body, c.TokenCount); err != nil {
			return err
		}
		if err := b.exec(`INSERT INTO chunks_fts VALUES(?,?,?,?,?,?,?)`, c.ChunkID, d.ID, d.Municipality, title, norm.NFC.String(c.Heading), norm.NFC.String(c.Body), et); err != nil {
			return err
		}
		b.texts = append(b.texts, BuildEmbeddingText(d, c))
		b.vectors = append(b.vectors, chromem.Document{ID: c.ChunkID, Content: c.Body, Metadata: map[string]string{"municipality": d.Municipality, "doc_id": d.ID, "chunk_id": c.ChunkID, "category": d.Category, "doc_type": d.DocType, "heading": c.Heading}})
		if len(b.texts) == 32 {
			if err := b.flush(); err != nil {
				return err
			}
		}
	}
	b.stats.TotalDocuments++
	b.stats.TotalChunks += len(chunks)
	b.stats.TotalWords += d.WordCount
	b.categories[d.Category]++
	return nil
}

func (b *builder) flush() error {
	if len(b.texts) == 0 {
		return nil
	}
	v, err := b.embedder.EmbedBatch(b.ctx, b.texts)
	if err != nil {
		return err
	}
	if len(v) != len(b.vectors) {
		return fmt.Errorf("expected %d vectors, got %d", len(b.vectors), len(v))
	}
	for i := range v {
		if len(v[i]) != b.embedder.Dimensions() || len(v[i]) == 0 {
			return fmt.Errorf("chunk %s: invalid embedding dimensions %d", b.vectors[i].ID, len(v[i]))
		}
		b.vectors[i].Embedding = v[i]
	}
	if err = b.col.AddDocuments(b.ctx, b.vectors, 1); err != nil {
		return err
	}
	b.texts = nil
	b.vectors = nil
	return nil
}
