package server

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/municipal-archives-india/manesar/internal/model"
)

type endpoint func(*http.Request) (any, error)
type badRequest string

func (e badRequest) Error() string { return string(e) }

func publicError(err error) (int, string) {
	var bad badRequest
	switch {
	case errors.As(err, &bad):
		return http.StatusBadRequest, bad.Error()
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound, "Archive record not found"
	default:
		slog.Error("archive request failed", "error", err)
		return http.StatusInternalServerError, "Archive request failed"
	}
}

func apiHandler(fn endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := fn(r)
		var body []byte
		status := http.StatusOK
		if err == nil {
			body, err = json.Marshal(payload)
		}
		if err != nil {
			code, msg := publicError(err)
			status = code
			body, _ = json.Marshal(map[string]string{"error": msg})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(append(body, '\n'))
	}
}

func parseRequest(r *http.Request) (model.SearchRequest, error) {
	var req model.SearchRequest
	if len(r.URL.RawQuery) > 32*1024 {
		return req, badRequest("Query parameters are too long")
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return req, badRequest("Invalid query parameters")
	}
	for _, key := range []string{"q", "mode", "sort", "limit", "municipality", "category", "doc_type", "ward"} {
		v := values[key]
		if len(v) > 1 {
			return req, badRequest("Repeated parameter: " + key)
		}
		max := 128
		if key == "q" {
			max = 2048
		}
		if len(v) == 1 && (!utf8.ValidString(v[0]) || utf8.RuneCountInString(v[0]) > max || strings.IndexFunc(v[0], unicode.IsControl) >= 0) {
			return req, badRequest("Invalid parameter: " + key)
		}
	}
	req.Query = strings.TrimSpace(values.Get("q"))
	req.Municipality = strings.TrimSpace(values.Get("municipality"))
	if req.Municipality == "" {
		req.Municipality = "manesar"
	}
	req.Category = strings.TrimSpace(values.Get("category"))
	req.DocType = strings.TrimSpace(values.Get("doc_type"))
	req.Sort = strings.TrimSpace(values.Get("sort"))
	switch req.Sort {
	case "", model.SortRelevance, model.SortDateDesc, model.SortDateAsc, model.SortTitle:
	default:
		return req, badRequest("sort must be relevance, date_desc, date_asc or title")
	}
	req.Mode = model.SearchMode(values.Get("mode"))
	switch req.Mode {
	case "":
		req.Mode = model.SearchModeHybrid
	case model.SearchModeHybrid, model.SearchModeVector, model.SearchModeKeyword:
	default:
		return req, badRequest("mode must be hybrid, vector or keyword")
	}
	req.Limit = 10
	if v := values.Get("limit"); v != "" {
		req.Limit, err = strconv.Atoi(v)
		if err != nil || req.Limit < 1 || req.Limit > model.MaxSearchLimit {
			return req, badRequest(fmt.Sprintf("limit must be an integer between 1 and %d", model.MaxSearchLimit))
		}
	}
	if v := values.Get("ward"); v != "" {
		req.Ward, err = strconv.Atoi(v)
		if err != nil || req.Ward < 1 {
			return req, badRequest("ward must be a positive integer")
		}
	}
	return req, nil
}

func queryRequest(r *http.Request) (model.SearchRequest, error) {
	req, err := parseRequest(r)
	if err == nil && req.Query == "" {
		err = badRequest("q is required")
	}
	return req, err
}

func searchRequest(r *http.Request) (model.SearchRequest, error) {
	req, err := queryRequest(r)
	if err == nil {
		req.Sort, err = searchSort(req.Sort)
	}
	return req, err
}

func searchSort(sort string) (string, error) {
	if sort == model.SortTitle {
		return "", badRequest("sort must be relevance, date_desc or date_asc")
	}
	return cmp.Or(sort, model.SortRelevance), nil
}

func indexSort(sort string) (string, error) {
	if sort == model.SortRelevance {
		return "", badRequest("sort must be date_desc, date_asc or title")
	}
	return cmp.Or(sort, model.SortDateDesc), nil
}

func (s *Server) search(r *http.Request) (any, error) {
	req, err := searchRequest(r)
	if err != nil {
		return nil, err
	}
	return s.svc.Search(r.Context(), req)
}

func (s *Server) ask(r *http.Request) (any, error) {
	req, err := searchRequest(r)
	if err != nil {
		return nil, err
	}
	return s.svc.Ask(r.Context(), req)
}

func (s *Server) suggest(r *http.Request) (any, error) {
	req, err := queryRequest(r)
	if err != nil {
		return nil, err
	}
	items, err := s.svc.Suggest(r.Context(), req.Municipality, req.Query, req.Limit)
	return map[string]any{"suggestions": list(items)}, err
}

func list[T any](rows []T) []T {
	if rows == nil {
		return []T{}
	}
	return rows
}

func (s *Server) municipalities(r *http.Request) (any, error) {
	rows, err := s.svc.ListMunicipalities(r.Context())
	return map[string]any{"municipalities": list(rows)}, err
}

func (s *Server) documents(r *http.Request) (any, error) {
	req, err := parseRequest(r)
	if err != nil {
		return nil, err
	}
	req.Sort, err = indexSort(req.Sort)
	if err != nil {
		return nil, err
	}
	rows, err := s.svc.ListDocumentsSorted(r.Context(), req.Municipality, req.Category, req.DocType, req.Ward, req.Sort)
	return map[string]any{"documents": list(rows)}, err
}

func (s *Server) wards(r *http.Request) (any, error) {
	m := r.PathValue("municipality")
	rows, err := s.svc.ListWards(r.Context(), m)
	return map[string]any{"municipality": m, "wards": list(rows)}, err
}

func (s *Server) officers(r *http.Request) (any, error) {
	m := r.PathValue("municipality")
	rows, err := s.svc.ListOfficers(r.Context(), m)
	return map[string]any{"municipality": m, "officers": list(rows)}, err
}

func (s *Server) readDocument(r *http.Request) (*model.ArchiveDocumentFull, template.HTML, error) {
	doc, err := s.svc.GetDocument(r.Context(), r.PathValue("municipality"), r.PathValue("id"))
	if err != nil {
		return nil, "", err
	}
	if doc == nil {
		return nil, "", fs.ErrNotExist
	}
	pathToID := map[string]string{}
	if docs, listErr := s.svc.ListDocuments(r.Context(), doc.Municipality, "", "", 0); listErr == nil {
		for _, item := range docs {
			if item.FilePath != "" {
				pathToID[item.FilePath] = item.ID
			}
		}
	}
	content, err := RenderMarkdownWithLinks(doc.Content, doc.Municipality, doc.FilePath, pathToID)
	if err != nil {
		return nil, "", fmt.Errorf("render document: %w", err)
	}
	result := *doc
	result.HTMLContent = string(content)
	return &result, content, nil
}

func (s *Server) document(r *http.Request) (any, error) {
	doc, _, err := s.readDocument(r)
	return doc, err
}
