package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/municipal-archives-india/manesar/internal/indexer"
	"github.com/municipal-archives-india/manesar/internal/model"
	"github.com/municipal-archives-india/manesar/internal/search"
	"github.com/municipal-archives-india/manesar/internal/server"
	"github.com/municipal-archives-india/manesar/internal/web"
	chromem "github.com/philippgille/chromem-go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	const usage = "usage: archive {index|serve|query} [flags]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Println(usage)
		return nil
	}
	cmd := args[0]
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	dataDir := f.String("data", "data", "index directory")
	models := f.String("models", ".cache/hugot-models", "embedding model directory")
	var archives, addr, mode, municipality, sortBy string
	var auto bool
	var limit int
	switch cmd {
	case "index", "serve":
		f.StringVar(&archives, "archives", "archives", "archive source directory")
		if cmd == "serve" {
			f.StringVar(&addr, "addr", ":8080", "HTTP listen address")
			f.BoolVar(&auto, "auto-index", true, "build index if no complete generation exists")
		}
	case "query":
		f.StringVar(&sortBy, "sort", model.SortRelevance, "relevance, date_desc or date_asc")
		f.StringVar(&mode, "mode", "hybrid", "hybrid, vector or keyword")
		f.StringVar(&municipality, "municipality", "manesar", "municipality slug (or all)")
		f.IntVar(&limit, "limit", 5, fmt.Sprintf("maximum results (1-%d)", model.MaxSearchLimit))
	default:
		return errors.New(usage)
	}
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(f.Args(), " "))
	if cmd == "query" {
		if query == "" {
			return errors.New("usage: archive query [flags] <question>")
		}
		if mode != "hybrid" && mode != "vector" && mode != "keyword" {
			return errors.New("mode must be hybrid, vector or keyword")
		}
		if limit < 1 || limit > model.MaxSearchLimit {
			return fmt.Errorf("limit must be between 1 and %d", model.MaxSearchLimit)
		}
		if sortBy != model.SortRelevance && sortBy != model.SortDateDesc && sortBy != model.SortDateAsc {
			return errors.New("sort must be relevance, date_desc or date_asc")
		}
	} else if f.NArg() != 0 {
		return fmt.Errorf("unexpected argument: %s", f.Arg(0))
	}

	dbPath, vectorDir, ready := indexer.ResolveDataPaths(*dataDir)
	build := cmd == "index"
	if !build && !ready {
		if cmd == "serve" && auto {
			build = true
		} else {
			return fmt.Errorf("no complete, compatible archive index found in %s (run `archive index` first)", *dataDir)
		}
	}

	needVectors := build || cmd == "serve" || mode != "keyword"
	var embedder model.Embedder
	if needVectors {
		emb, err := indexer.NewHugotEmbedder(ctx, *models)
		if err != nil {
			return err
		}
		defer emb.Close()
		embedder = emb
	}
	if build {
		start := time.Now()
		stats, err := indexer.BuildIndex(ctx, archives, *dataDir, embedder)
		if err != nil {
			return err
		}
		fmt.Printf("Indexed %d municipalities, %d documents, %d chunks and %d words (%d vector dimensions) in %s.\n",
			stats.MunicipalitiesIndexed, stats.TotalDocuments, stats.TotalChunks, stats.TotalWords, stats.EmbeddingDimensions, time.Since(start).Round(time.Millisecond))
		if cmd == "index" {
			return nil
		}
		dbPath, vectorDir, ready = indexer.ResolveDataPaths(*dataDir)
		if !ready {
			return errors.New("index completed without publishing a readable generation")
		}
	}

	db, err := indexer.OpenSQLite(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var vectors *chromem.DB
	if needVectors {
		vectors, err = indexer.OpenVectorDB(vectorDir)
		if err != nil {
			return err
		}
	}
	engine, err := search.NewEngine(db, vectors, embedder)
	if err != nil {
		return err
	}
	if cmd == "query" {
		answer, err := engine.Ask(ctx, model.SearchRequest{Query: query, Municipality: municipality, Mode: model.SearchMode(mode), Sort: sortBy, Limit: limit})
		if err != nil {
			return err
		}
		fmt.Println(answer.Brief.Headline)
		for _, p := range answer.Brief.AnswerParagraphs {
			fmt.Println(p)
		}
		fmt.Println("\nCitations:")
		for _, c := range answer.Brief.Citations {
			fmt.Printf("[%d] %s (%s)\n    %s\n", c.Index, c.Title, c.DocID, c.SourceURL)
		}
		fmt.Println("\nTop hits:")
		for i, h := range answer.Hits {
			fmt.Printf("%d. [%s] %s (%s, %.3f)\n", i+1, h.Municipality, h.Title, h.DocID, h.Score)
		}
		return nil
	}
	t, err := web.LoadTemplates(server.TemplateFuncs())
	if err != nil {
		return err
	}
	handler, err := server.New(engine, server.Options{Templates: t, StaticFS: web.StaticFS()})
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	return serve(ctx, srv)
}

func serve(ctx context.Context, srv *http.Server) error {
	done := make(chan error, 1)
	go func() { log.Printf("Starting archive HTTP server on %s", srv.Addr); done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			_ = srv.Close()
			return err
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
