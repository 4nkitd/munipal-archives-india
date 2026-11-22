package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/municipal-archives-india/manesar/internal/indexer"
	"golang.org/x/text/unicode/norm"
)

func audit(root, data string, manifest io.Writer) error {
	dbPath, vectors, ready := indexer.ResolveDataPaths(data)
	if !ready {
		return errors.New("no complete, compatible index")
	}
	location, err := filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	dsn := url.URL{Scheme: "file", Path: location, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	vectorDB, err := indexer.OpenVectorDB(vectors)
	if err != nil {
		return err
	}
	collection := vectorDB.GetCollection(indexer.CollectionName, nil)
	if collection == nil {
		return errors.New("missing vector collection")
	}

	nDocs, nChunks := 0, 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		doc, err := indexer.ParseMarkdownFile(path, root)
		if err != nil {
			return err
		}
		chunks := indexer.ChunkDocument(doc)
		if len(chunks) == 0 || len(doc.Contacts) != 0 || contactLike(doc.Content) {
			return fmt.Errorf("%s: public document failed the contact or searchability check", doc.FilePath)
		}
		var indexedPath, title, body, summary, source, tagsJSON string
		var count, contactCount int
		err = db.QueryRowContext(ctx, `SELECT file_path, title, content, summary, source_url, tags_json, chunk_count, json_array_length(contacts_json)
			FROM documents WHERE municipality=? AND id=?`, doc.Municipality, doc.ID).
			Scan(&indexedPath, &title, &body, &summary, &source, &tagsJSON, &count, &contactCount)
		if err != nil {
			return fmt.Errorf("%s: %w", doc.FilePath, err)
		}
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			return fmt.Errorf("%s: %w", doc.FilePath, err)
		}
		if indexedPath != doc.FilePath || title != doc.Title || body != doc.Content || summary != doc.Summary ||
			source != doc.SourceURL || count != len(chunks) || contactCount != 0 || !slices.Equal(tags, doc.Tags) {
			return fmt.Errorf("%s: index differs from public record", doc.FilePath)
		}
		for _, chunk := range chunks {
			var body, municipality, docID, ftsTitle, ftsHeading, ftsBody, ftsEntities string
			err := db.QueryRowContext(ctx, `SELECT c.body,c.municipality,c.doc_id,f.title,f.heading,f.body,f.entities
				FROM chunks c JOIN chunks_fts f USING(chunk_id)
				WHERE c.chunk_id=? AND c.municipality=f.municipality AND c.doc_id=f.doc_id`, chunk.ChunkID).
				Scan(&body, &municipality, &docID, &ftsTitle, &ftsHeading, &ftsBody, &ftsEntities)
			if err != nil {
				return fmt.Errorf("%s: %w", chunk.ChunkID, err)
			}
			vector, err := collection.GetByID(ctx, chunk.ChunkID)
			if err != nil {
				return fmt.Errorf("%s: %w", chunk.ChunkID, err)
			}
			if body != chunk.Body || vector.Content != body || municipality != doc.Municipality || docID != doc.ID ||
				vector.Metadata["municipality"] != municipality || vector.Metadata["doc_id"] != docID || len(vector.Embedding) != 384 ||
				ftsTitle != norm.NFC.String(doc.Title) || ftsHeading != norm.NFC.String(chunk.Heading) ||
				ftsBody != norm.NFC.String(body) || contactLike(ftsEntities) {
				return fmt.Errorf("%s: passage differs between source, full-text and vector stores", chunk.ChunkID)
			}
		}
		nDocs++
		nChunks += len(chunks)
		_, err = fmt.Fprintf(manifest, "%s\t%s\n", doc.Municipality, doc.ID)
		return err
	})
	if err != nil {
		return err
	}
	var docCount, chunkCount, ftsCount, filledContacts int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM documents), (SELECT count(*) FROM chunks),
		(SELECT count(*) FROM chunks_fts), (SELECT count(*) FROM (
			SELECT profile_json AS payload FROM municipalities UNION ALL
			SELECT record_json FROM wards UNION ALL SELECT record_json FROM officers
		) a, json_tree(a.payload) j WHERE lower(j.key) IN ('phone', 'email', 'helpdeskphone', 'tollfree', 'whatsapp')
		AND j.type='text' AND trim(j.value) != '')`).Scan(&docCount, &chunkCount, &ftsCount, &filledContacts); err != nil {
		return err
	}
	if nDocs != docCount || nChunks != chunkCount || nChunks != ftsCount || nChunks != collection.Count() || filledContacts != 0 {
		return fmt.Errorf("index totals differ from the public source (records %d/%d, chunks %d/%d, fts %d, vectors %d, nonempty direct contact fields %d)",
			nDocs, docCount, nChunks, chunkCount, ftsCount, collection.Count(), filledContacts)
	}
	fmt.Fprintf(os.Stderr, "Audited %d public records and %d passages in SQLite, FTS and vectors.\n", nDocs, nChunks)
	return nil
}
