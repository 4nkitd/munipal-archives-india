package indexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
	"github.com/municipal-archives-india/manesar/internal/model"
)

const bgeModel = "BAAI/bge-small-en-v1.5"

type embedWorker struct {
	mu      sync.Mutex
	session *hugot.Session
	pipe    *pipelines.FeatureExtractionPipeline
}

type HugotEmbedder struct {
	mu      sync.RWMutex
	closed  bool
	workers []*embedWorker
}

var _ model.Embedder = (*HugotEmbedder)(nil)

func NewHugotEmbedder(ctx context.Context, modelsDir string) (*HugotEmbedder, error) {
	dir := filepath.Join(modelsDir, "BAAI_bge-small-en-v1.5")
	found := false
	for _, name := range []string{"onnx/model.onnx", "model.onnx"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.Mode().IsRegular() {
			found = true
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if !found {
		opts := hugot.NewDownloadOptions()
		opts.OnnxFilePath = "onnx/model.onnx"
		path, err := hugot.DownloadModel(ctx, bgeModel, modelsDir, opts)
		if err != nil {
			return nil, err
		}
		dir = path
	}
	// GoMLX already parallelizes tensor ops across CPU cores internally; a single
	// session avoids thread oversubscription and keeps server memory footprint small.
	nWorkers := 1
	workers := make([]*embedWorker, 0, nWorkers)
	cleanup := func() {
		for _, w := range workers {
			if w.session != nil {
				_ = w.session.Destroy()
			}
		}
	}
	for i := 0; i < nWorkers; i++ {
		s, err := hugot.NewGoSession(ctx)
		if err != nil {
			cleanup()
			return nil, err
		}
		p, err := hugot.NewPipeline(s, hugot.FeatureExtractionConfig{
			ModelPath:    dir,
			Name:         fmt.Sprintf("bge-small-%d", i),
			OnnxFilename: "model.onnx",
			Options:      []hugot.FeatureExtractionOption{pipelines.WithNormalization()},
		})
		if err != nil {
			_ = s.Destroy()
			cleanup()
			return nil, err
		}
		workers = append(workers, &embedWorker{session: s, pipe: p})
	}
	return &HugotEmbedder{workers: workers}, nil
}

func (*HugotEmbedder) ModelName() string { return bgeModel }
func (*HugotEmbedder) Dimensions() int   { return 384 }

func (e *HugotEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	var errs []error
	for _, w := range e.workers {
		w.mu.Lock()
		if w.session != nil {
			errs = append(errs, w.session.Destroy())
			w.session = nil
			w.pipe = nil
		}
		w.mu.Unlock()
	}
	return errors.Join(errs...)
}

func (e *HugotEmbedder) runOn(ctx context.Context, w *embedWorker, input []string) ([][]float32, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if w.pipe == nil {
		return nil, errors.New("embedder closed")
	}
	r, err := w.pipe.RunPipeline(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(r.Embeddings) != len(input) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(input), len(r.Embeddings))
	}
	return r.Embeddings, nil
}

func (e *HugotEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || len(e.workers) == 0 {
		return nil, errors.New("embedder closed")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	const subBatch = 8
	if len(texts) <= subBatch || len(e.workers) == 1 {
		out := make([][]float32, 0, len(texts))
		for start := 0; start < len(texts); start += subBatch {
			end := min(len(texts), start+subBatch)
			input := make([]string, end-start)
			for i := range input {
				input[i] = capped(texts[start+i], 1300)
			}
			vecs, err := e.runOn(ctx, e.workers[0], input)
			if err != nil {
				return nil, err
			}
			out = append(out, vecs...)
		}
		return out, nil
	}

	type job struct {
		start int
		input []string
	}
	jobs := make(chan job, (len(texts)+subBatch-1)/subBatch)
	for start := 0; start < len(texts); start += subBatch {
		end := min(len(texts), start+subBatch)
		input := make([]string, end-start)
		for i := range input {
			input[i] = capped(texts[start+i], 1300)
		}
		jobs <- job{start: start, input: input}
	}
	close(jobs)

	out := make([][]float32, len(texts))
	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	for _, w := range e.workers {
		wg.Add(1)
		go func(worker *embedWorker) {
			defer wg.Done()
			for j := range jobs {
				errMu.Lock()
				failed := firstErr != nil
				errMu.Unlock()
				if failed || ctx.Err() != nil {
					return
				}
				vecs, err := e.runOn(ctx, worker, j.input)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					return
				}
				copy(out[j.start:j.start+len(vecs)], vecs)
			}
		}(w)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, ctx.Err()
}

func (e *HugotEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{"Represent this sentence for searching relevant passages: " + query})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}
