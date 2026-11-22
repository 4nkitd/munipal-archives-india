package search

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/municipal-archives-india/manesar/internal/model"
)

func (e *Engine) Suggest(ctx context.Context, municipal, prefix string, limit int) ([]model.SuggestItem, error) {
	if limit == 0 {
		limit = 8
	}
	limit = max(1, min(limit, 50))
	p := normalized(prefix)
	type suggestion struct {
		x        model.SuggestItem
		priority int
	}
	var pool []suggestion
	seen := map[string]bool{}
	add := func(x model.SuggestItem, extra string) {
		name := normalized(x.Label)
		priority := 2
		switch {
		case name == p:
			priority = 0
		case strings.HasPrefix(name, p):
			priority = 1
		case strings.Contains(name, p):
		case strings.Contains(normalized(x.Sublabel+" "+extra), p):
			priority = 3
		default:
			return
		}
		key := x.Type + "|" + name + "|" + x.Municipality + "|" + x.DocID
		if !seen[key] {
			seen[key] = true
			pool = append(pool, suggestion{x, priority})
		}
	}
	if len([]rune(p)) <= 3 {
		for _, q := range []string{"Who is the Junior Engineer for Ward 9?", "How to submit a Property ID objection", "Where can I file a sanitation complaint?", "Tenders and works in Ward 6", "Birth certificate application deadline"} {
			add(model.SuggestItem{Type: "prompt", Label: q, Sublabel: "Citizen question", Query: q}, "")
		}
	}
	ws, err := e.ListWards(ctx, municipal)
	if err != nil {
		return nil, err
	}
	byNum := map[int]bool{}
	for _, w := range ws {
		byNum[w.WardNumber] = true
		name := fmt.Sprintf("Ward %d", w.WardNumber)
		desc := strings.Join(w.Villages, ", ")
		add(model.SuggestItem{Type: "ward", Label: name, Ward: w.WardNumber, Sublabel: cmp.Or(desc, w.Name), Query: name}, strings.Join(w.Sectors, " "))
		for _, v := range w.Villages {
			add(model.SuggestItem{Type: "village", Label: v, Sublabel: name, Query: v, Ward: w.WardNumber}, "")
		}
		for _, s := range w.Sectors {
			label := "Sector " + sector(s)
			add(model.SuggestItem{Type: "sector", Label: label, Sublabel: name, Query: label, Ward: w.WardNumber}, s)
		}
	}
	if municipality(municipal) == "manesar" || municipal == "all" {
		for i := 1; i <= 20; i++ {
			if !byNum[i] {
				name := fmt.Sprintf("Ward %d", i)
				add(model.SuggestItem{Type: "ward", Label: name, Query: name, Ward: i}, "")
			}
		}
		for _, v := range villages {
			add(model.SuggestItem{Type: "village", Label: v, Query: v}, "")
		}
	}
	os, err := e.ListOfficers(ctx, municipal)
	if err != nil {
		return nil, err
	}
	for _, o := range os {
		var labels []string
		for _, w := range o.Wards {
			labels = append(labels, "Ward "+strconv.Itoa(w))
		}
		desc := strings.Join(append([]string{o.Designation}, labels...), ", ")
		add(model.SuggestItem{Type: "officer", Label: o.Name, Sublabel: desc, Query: o.Name}, o.Branch)
	}
	m := scope(municipal)
	rows, err := e.db.QueryContext(ctx, `SELECT municipality,id,title,category FROM documents WHERE (?='' OR municipality=?) AND instr(lower(title),?)>0 ORDER BY CASE WHEN instr(lower(title),?)=1 THEN 0 ELSE 1 END,title,municipality,id LIMIT ?`, m, m, strings.ToLower(strings.TrimSpace(prefix)), strings.ToLower(strings.TrimSpace(prefix)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		x := model.SuggestItem{Type: "document"}
		if err := rows.Scan(&x.Municipality, &x.DocID, &x.Label, &x.Sublabel); err != nil {
			return nil, err
		}
		x.Query = x.Label
		add(x, "")
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(pool, func(a, b suggestion) int { return cmp.Compare(a.priority, b.priority) })
	out := make([]model.SuggestItem, 0, min(limit, len(pool)))
	for _, s := range pool[:cap(out)] {
		out = append(out, s.x)
	}
	return out, nil
}
