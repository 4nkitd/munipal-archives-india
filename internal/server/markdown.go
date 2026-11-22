package server

import (
	"bytes"
	"html/template"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

// RenderMarkdown converts Markdown to safe HTML (no raw HTML execution).
func RenderMarkdown(md string) (template.HTML, error) {
	return RenderMarkdownWithLinks(md, "", "", nil)
}

// RenderMarkdownWithLinks converts Markdown to safe HTML and rewrites relative
// archive .md links against the municipality's file_path -> doc_id table.
func RenderMarkdownWithLinks(md, municipality, sourceFilePath string, pathToID map[string]string) (template.HTML, error) {
	src := []byte(md)
	doc := markdown.Parser().Parse(text.NewReader(src))
	if len(pathToID) > 0 && municipality != "" && sourceFilePath != "" {
		baseDir := path.Dir(sourceFilePath)
		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			lnk, ok := n.(*ast.Link)
			if !ok {
				return ast.WalkContinue, nil
			}
			dest := string(lnk.Destination)
			if dest == "" || strings.HasPrefix(dest, "#") || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "tel:") || strings.HasPrefix(dest, "/") {
				return ast.WalkContinue, nil
			}
			filePart, frag, hasFrag := strings.Cut(dest, "#")
			if !strings.HasSuffix(strings.ToLower(filePart), ".md") {
				return ast.WalkContinue, nil
			}
			resolved := path.Clean(path.Join(baseDir, filePart))
			if id, found := pathToID[resolved]; found {
				canonical := "/doc/" + url.PathEscape(municipality) + "/" + url.PathEscape(id)
				if hasFrag && frag != "" {
					canonical += "#" + frag
				}
				lnk.Destination = []byte(canonical)
			}
			return ast.WalkContinue, nil
		})
	}
	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, src, doc); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
