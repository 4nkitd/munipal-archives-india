package indexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/municipal-archives-india/manesar/internal/model"
)

func ChunkDocument(doc *model.ArchiveDocument) []model.DocumentChunk {
	out := []model.DocumentChunk{}
	heading, parent, body := "", "", ""
	count := 0
	flush := func() {
		if count == 0 {
			return
		}
		out = append(out, model.DocumentChunk{DocID: doc.ID, Municipality: doc.Municipality, Heading: heading, Body: strings.TrimSpace(body), TokenCount: count})
		body = ""
		count = 0
	}
	appendUnit := func(line string, n int) {
		if count+n > 340 {
			words := strings.Fields(body)
			overlap := words[max(0, len(words)-40):]
			flush()
			body = strings.Join(overlap, " ")
			count = len(overlap)
		}
		body += "\n" + line
		count += n
	}
	for _, line := range strings.Split(doc.Content, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "### ") {
			flush()
			if h, ok := strings.CutPrefix(t, "## "); ok {
				parent = h
				heading = h
			} else {
				heading = strings.TrimPrefix(t, "### ")
				if parent != "" {
					heading = parent + " / " + heading
				}
			}
			continue
		}
		words := strings.Fields(t)
		if len(words) == 0 {
			if body != "" && !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			continue
		}
		if len(words) <= 300 {
			appendUnit(line, len(words))
			continue
		}
		for len(words) > 0 {
			n := min(220, len(words))
			appendUnit(strings.Join(words[:n], " "), n)
			words = words[n:]
		}
	}
	flush()
	for i := 0; i < len(out) && len(out) > 1; {
		if out[i].TokenCount >= 30 {
			i++
			continue
		}
		j := i - 1
		if i == 0 || (i+1 < len(out) && out[i+1].TokenCount < out[j].TokenCount) {
			j = i + 1
		}
		small := out[i].Body
		if out[i].Heading != "" && out[i].Heading != out[j].Heading {
			small = out[i].Heading + "\n" + small
		}
		if j < i {
			out[j].Body += "\n\n" + small
		} else {
			out[j].Body = small + "\n\n" + out[j].Body
		}
		out[j].TokenCount = len(strings.Fields(out[j].Body))
		out = append(out[:i], out[i+1:]...)
	}
	for i := range out {
		out[i].ChunkIndex = i
		out[i].ChunkID = fmt.Sprintf("%s#%03d", doc.ID, i)
	}
	return out
}

func capped(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

func geography(doc *model.ArchiveDocument) string {
	var parts []string
	for _, w := range doc.Wards {
		parts = append(parts, fmt.Sprintf("Ward %d", w))
	}
	parts = append(parts, doc.Villages...)
	parts = append(parts, doc.Sectors...)
	return strings.Join(parts, ", ")
}

func BuildEmbeddingText(doc *model.ArchiveDocument, chunk model.DocumentChunk) string {
	header := strings.Join([]string{doc.Title, doc.Category, doc.Department, capped(geography(doc), 300), chunk.Heading}, "\n")
	header = capped(header, 700)
	return header + "\n" + capped(chunk.Body, 1799-utf8.RuneCountInString(header))
}
