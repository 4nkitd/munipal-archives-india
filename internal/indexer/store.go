package indexer

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	chromem "github.com/philippgille/chromem-go"
	_ "modernc.org/sqlite"
)

const (
	SQLiteFilename  = "archive.db"
	VectorDirname   = "vectors"
	CurrentFilename = "CURRENT"
	GenerationsDir  = "generations"
	CompleteMarker  = "COMPLETE"
)

// ResolveDataPaths returns the SQLite DB path, chromem vector directory, and whether
// a complete committed generation exists. Supports atomic generation pointers in
// dataDir/CURRENT and falls back to legacy top-level archive.db + vectors.
func ResolveDataPaths(dataDir string) (dbPath string, vectorDir string, ready bool) {
	if raw, err := os.ReadFile(filepath.Join(dataDir, CurrentFilename)); err == nil {
		gen := strings.TrimSpace(string(raw))
		if gen != "" && !strings.Contains(gen, "..") && !filepath.IsAbs(gen) {
			genDir := filepath.Join(dataDir, gen)
			db := filepath.Join(genDir, SQLiteFilename)
			vec := filepath.Join(genDir, VectorDirname)
			if fileExists(filepath.Join(genDir, CompleteMarker)) && fileExists(db) && hasVectorData(vec) {
				return db, vec, true
			}
		}
	}
	legacyDB := filepath.Join(dataDir, SQLiteFilename)
	legacyVec := filepath.Join(dataDir, VectorDirname)
	if fileExists(legacyDB) && hasVectorData(legacyVec) && hasCommittedIndex(legacyDB) {
		return legacyDB, legacyVec, true
	}
	return legacyDB, legacyVec, false
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func hasVectorData(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}

func hasCommittedIndex(dbPath string) bool {
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	var indexedAt string
	if err := db.QueryRow(`SELECT value FROM index_meta WHERE key='indexed_at'`).Scan(&indexedAt); err != nil {
		return false
	}
	return strings.TrimSpace(indexedAt) != ""
}

func OpenSQLite(dbPath string) (*sql.DB, error) {
	dsn := dbPath
	if dbPath != ":memory:" {
		abs, err := filepath.Abs(dbPath)
		if err != nil {
			return nil, err
		}
		u := url.URL{Scheme: "file", Path: abs}
		dsn = u.String()
	}
	db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	}
	if err = InitSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS municipalities(slug TEXT PRIMARY KEY, profile_json TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS documents(
id TEXT NOT NULL, municipality TEXT NOT NULL, title TEXT NOT NULL, category TEXT NOT NULL, subcategory TEXT NOT NULL, doc_type TEXT NOT NULL,
date TEXT NOT NULL, source_url TEXT NOT NULL, source_portal TEXT NOT NULL, original_filename TEXT NOT NULL, department TEXT NOT NULL,
wards_json TEXT NOT NULL, sectors_json TEXT NOT NULL, villages_json TEXT NOT NULL, tags_json TEXT NOT NULL, contacts_json TEXT NOT NULL,
action_url TEXT NOT NULL, summary TEXT NOT NULL, verified_at TEXT NOT NULL, file_path TEXT NOT NULL, word_count INTEGER NOT NULL,
chunk_count INTEGER NOT NULL, content TEXT NOT NULL, PRIMARY KEY(municipality,id));
CREATE TABLE IF NOT EXISTS chunks(chunk_id TEXT PRIMARY KEY, doc_id TEXT NOT NULL, municipality TEXT NOT NULL, chunk_index INTEGER NOT NULL, heading TEXT NOT NULL, body TEXT NOT NULL, token_count INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS chunks_by_document ON chunks(municipality,doc_id,chunk_index);
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(chunk_id UNINDEXED, doc_id UNINDEXED, municipality UNINDEXED, title, heading, body, entities, tokenize='porter unicode61');
CREATE TABLE IF NOT EXISTS wards(municipality TEXT NOT NULL, ward_number INTEGER NOT NULL, record_json TEXT NOT NULL, PRIMARY KEY(municipality,ward_number));
CREATE TABLE IF NOT EXISTS officers(municipality TEXT NOT NULL, sr_no INTEGER NOT NULL, record_json TEXT NOT NULL, PRIMARY KEY(municipality,sr_no));
CREATE TABLE IF NOT EXISTS index_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL);`)
	return err
}

func OpenVectorDB(vectorDir string) (*chromem.DB, error) {
	return chromem.NewPersistentDB(vectorDir, false)
}
