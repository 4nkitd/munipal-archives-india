package server

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/municipal-archives-india/manesar/internal/model"
)

type Options struct {
	StaticFS  fs.FS
	Templates *template.Template
}

type Server struct {
	svc  model.SearchService
	tmpl *template.Template
	mux  *http.ServeMux
}

func New(svc model.SearchService, opts Options) (*Server, error) {
	if svc == nil || opts.Templates == nil || opts.StaticFS == nil {
		return nil, errors.New("archive server requires a search service, templates, and static files")
	}
	s := &Server{svc: svc, tmpl: opts.Templates, mux: http.NewServeMux()}
	static := http.StripPrefix("/static/", http.FileServerFS(opts.StaticFS))
	s.mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	})
	for pattern, fn := range map[string]endpoint{
		"GET /api/health": func(*http.Request) (any, error) {
			return map[string]string{"status": "ok", "engine": "go-chromem-fts5"}, nil
		},
		"GET /api/stats":                         func(r *http.Request) (any, error) { return s.svc.Stats(r.Context()) },
		"GET /api/municipalities":                s.municipalities,
		"GET /api/search":                        s.search,
		"GET /api/ask":                           s.ask,
		"GET /api/suggest":                       s.suggest,
		"GET /api/documents":                     s.documents,
		"GET /api/documents/{municipality}/{id}": s.document,
		"GET /api/wards/{municipality}":          s.wards,
		"GET /api/officers/{municipality}":       s.officers,
		"GET /api/":                              func(*http.Request) (any, error) { return nil, fs.ErrNotExist },
	} {
		s.mux.HandleFunc(pattern, apiHandler(fn))
	}
	for pattern, name := range map[string]string{
		"GET /{$}":                     "home",
		"GET /search":                  "search",
		"GET /doc/{municipality}/{id}": "document",
		"GET /wards":                   "wards",
		"GET /index":                   "index",
	} {
		s.mux.HandleFunc(pattern, s.page(name))
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	s.mux.ServeHTTP(w, r)
	slog.Info("archive request", "method", r.Method, "route", r.Pattern, "duration", time.Since(start))
}

func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDate": func(s string) string {
			s = strings.TrimSpace(s)
			if s == "" {
				return "Undated"
			}
			if d, err := time.Parse(time.DateOnly, s); err == nil {
				return d.Format("02 Jan 2006")
			}
			if d, err := time.Parse("2006-01", s); err == nil {
				return d.Format("Jan 2006")
			}
			return s
		},
		"publicationDay": model.PublicationDay,
		"registerTerms":  registerTerms,
		"lowerTitle":     strings.ToLower,
		"categoryLabel": func(s string) string {
			v := []rune(strings.NewReplacer("-", " ", "_", " ").Replace(s))
			if len(v) > 0 {
				v[0] = unicode.ToUpper(v[0])
			}
			return string(v)
		},
		"join": strings.Join,
		"joinInts": func(v []int, sep string) string {
			p := make([]string, len(v))
			for i, n := range v {
				p[i] = strconv.Itoa(n)
			}
			return strings.Join(p, sep)
		},
		"pct":         func(n float64) string { return fmt.Sprintf("%.0f%%", n*100) },
		"add":         func(a, b int) int { return a + b },
		"hasPrefix":   strings.HasPrefix,
		"formatScore": func(n float64) string { return fmt.Sprintf("%.3f", n) },
		"dict": func(args ...any) (map[string]any, error) {
			m := make(map[string]any)
			if len(args)%2 != 0 {
				return nil, errors.New("dict needs key/value pairs")
			}
			for i := 0; i < len(args); i += 2 {
				k, ok := args[i].(string)
				if !ok {
					return nil, errors.New("dict key must be a string")
				}
				m[k] = args[i+1]
			}
			return m, nil
		},
		"truncate": func(s string, n int) string {
			v := []rune(s)
			if n <= 0 {
				return ""
			}
			if len(v) <= n {
				return s
			}
			return string(v[:n-1]) + "…"
		},
		"safeURL":         safeURL,
		"renderParagraph": renderParagraph,
	}
}

var citeMarkerPattern = regexp.MustCompile(`\[(\d+)\]`)

func renderParagraph(text string, citations []model.PromptedCitation) template.HTML {
	byIdx := make(map[int]model.PromptedCitation, len(citations))
	for _, c := range citations {
		byIdx[c.Index] = c
	}
	var b strings.Builder
	last := 0
	for _, loc := range citeMarkerPattern.FindAllStringSubmatchIndex(text, -1) {
		b.WriteString(template.HTMLEscapeString(text[last:loc[0]]))
		numStr := text[loc[2]:loc[3]]
		idx, _ := strconv.Atoi(numStr)
		if c, ok := byIdx[idx]; ok {
			href := ""
			dataAttr := ""
			extra := ""
			if c.DocID != "" && c.Municipality != "" {
				href = "/doc/" + url.PathEscape(c.Municipality) + "/" + url.PathEscape(c.DocID)
				dataAttr = ` data-open-doc`
			} else if safe := string(safeURL(c.SourceURL)); safe != "" {
				href = safe
				if strings.HasPrefix(safe, "http://") || strings.HasPrefix(safe, "https://") {
					extra = ` target="_blank" rel="noopener noreferrer"`
				}
			}
			if href != "" {
				fmt.Fprintf(&b, `<a class="cite"%s href="%s"%s title="%s" aria-label="Source %d: %s">[%d]</a>`,
					dataAttr,
					template.HTMLEscapeString(href),
					extra,
					template.HTMLEscapeString(c.Title),
					c.Index,
					template.HTMLEscapeString(c.Title),
					c.Index,
				)
			} else {
				b.WriteString(template.HTMLEscapeString(text[loc[0]:loc[1]]))
			}
		} else {
			b.WriteString(template.HTMLEscapeString(text[loc[0]:loc[1]]))
		}
		last = loc[1]
	}
	b.WriteString(template.HTMLEscapeString(text[last:]))
	return template.HTML(b.String())
}

func safeURL(raw string) template.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, `\`) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return ""
	}
	// Permit relative app-internal paths (e.g. /wards?municipality=manesar), disallow protocol-relative //.
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return template.URL(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "https", "http":
		if u.Hostname() == "" {
			return ""
		}
	case "mailto", "tel":
		if u.Opaque == "" {
			return ""
		}
	default:
		return ""
	}
	return template.URL(raw)
}
