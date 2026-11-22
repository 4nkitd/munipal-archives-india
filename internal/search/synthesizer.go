package search

import (
	"cmp"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/municipal-archives-india/manesar/internal/model"
)

var facts = []struct {
	kind, label string
	pattern     *regexp.Regexp
}{
	{"deadline", "Time period", regexp.MustCompile(`(?i)\b\d+(?:\s*[-–]\s*\d+)?\s+(?:(?:working|calendar|business)\s+)?days?\b`)},
	{"amount", "Amount", regexp.MustCompile(`(?i)(?:\bRs\.?|\bINR|₹)\s*\d[\d,]*(?:\.\d+)?(?:\s+(?:lakhs?|crores?))?\b`)},
	{"date", "Date", regexp.MustCompile(`(?i)\b(?:\d{4}-\d{2}-\d{2}|\d{1,2}[-/.]\d{1,2}[-/.]\d{4}|\d{1,2}\s+(?:jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+\d{4})\b`)},
	{"phone", "Contact number", regexp.MustCompile(`\b(?:1800[- ]?\d{3,4}[- ]?\d{3,4}|0\d{2,4}[- ]\d{6,8}|[6-9]\d{9})\b`)},
}

var (
	endSentence    = regexp.MustCompile(`[!?]\s+|\.\s+|\n+`)
	linkPattern    = regexp.MustCompile(`(?i)\b(?:https?://[^\s<>()\[\]"]+|(?:[a-z0-9-]+\.)+(?:gov\.in|mcmanesar\.in|investharyana\.in|ulbcluhry\.org)(?:/[^\s<>()\[\]"]*)?|mcmanesar\.in(?:/[^\s<>()\[\]"]*)?)`)
	mdLinkPattern  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdTableSep     = regexp.MustCompile(`^\s*\|?(?:\s*:?-+:?\s*\|)+\s*:?-+:?\s*\|?\s*$`)
	mdFormatMarker = regexp.MustCompile(`[*_` + "`" + `]+`)
	directoryCue   = regexp.MustCompile(`(?i)\b(?:councillor|corporator|engineer|officer|commissioner|mayor|who\s+is|who\s+handles|contact|phone|number|helpline|zone|division|which\s+ward|covers|ward\s+directory|ward\s+office)\b`)
)

var trustedPortalHosts = map[string]bool{
	"mcmanesar.in":         true,
	"etenders.hry.nic.in":  true,
	"investharyana.in":     true,
	"ulbcluhry.org":        true,
	"mcfbd.org":            true,
	"mcfaridabad.in":       true,
	"mcpanchkula.org":      true,
	"mcpanchkula.in":       true,
	"mcambala.in":          true,
	"cityfinance.in":       true,
	"mcfarukhnagar.in":     true,
	"mckarnal.co.in":       true,
	"nagarnigampanipat.in": true,
	"mcrewari.in":          true,
	"mcrohtak.in":          true,
	"hsiidc.org.in":        true,
	"mcsonipat.in":         true,
	"smda.org.in":          true,
	"mcynr.com":            true,
}

func official(raw string) (string, bool) {
	raw = strings.TrimRight(raw, ".,;:")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	h := strings.ToLower(u.Hostname())
	bare := strings.TrimPrefix(h, "www.")
	if trustedPortalHosts[bare] || strings.HasSuffix(h, ".gov.in") || strings.HasSuffix(h, ".nic.in") || strings.HasSuffix(h, ".mcmanesar.in") || strings.HasSuffix(h, ".mcfbd.org") || strings.HasSuffix(h, ".mcfaridabad.in") || strings.HasSuffix(h, ".mcpanchkula.org") || strings.HasSuffix(h, ".mcpanchkula.in") {
		return u.String(), true
	}
	return "", false
}

func cleanMarkdownLine(s string) string {
	s = mdLinkPattern.ReplaceAllString(s, "$1")
	s = mdFormatMarker.ReplaceAllString(s, "")
	if strings.Contains(s, "|") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(s), "|"), "|")
		var nonEmpty []string
		for _, c := range cells {
			c = strings.TrimSpace(c)
			if c != "" {
				nonEmpty = append(nonEmpty, c)
			}
		}
		s = strings.Join(nonEmpty, "; ")
	}
	return strings.TrimSpace(strings.TrimLeft(plain(s), "#* |-"))
}

func sentences(s string) []string {
	var cleanedLines []string
	for _, rawLine := range strings.Split(s, "\n") {
		if mdTableSep.MatchString(rawLine) {
			continue
		}
		cleanedLines = append(cleanedLines, rawLine)
	}
	s = strings.Join(cleanedLines, "\n")
	var out []string
	start := 0
	for _, p := range endSentence.FindAllStringIndex(s, -1) {
		i := p[0]
		if s[i] == '.' {
			prefix := mdFormatMarker.ReplaceAllString(s[start:i], "")
			last := strings.Fields(strings.ToLower(prefix))
			if len(last) > 0 && slices.Contains([]string{"rs", "sh", "shri", "dr", "sr", "no", "smt", "mr", "ms", "vs", "sec", "st"}, last[len(last)-1]) {
				continue
			}
		}
		end := i
		if s[i] != '\n' {
			end++
		}
		out = append(out, s[start:end])
		start = p[1]
	}
	return append(out, s[start:])
}

func contactText(c model.ContactInfo) string {
	t := c.Role
	if c.Name != "" {
		t += ": " + c.Name
	}
	for _, s := range []string{c.Phone, c.Email} {
		if s != "" {
			t += ", " + s
		}
	}
	return strings.Trim(t, ", ")
}

func BuildPromptedAnswer(query string, hits []model.SearchHit, entities model.DetectedEntities, wards []model.WardRecord, officers []model.OfficerRecord) model.PromptedAnswer {
	return BuildPromptedAnswerForMunicipality(query, "", hits, entities, wards, officers)
}

func BuildPromptedAnswerForMunicipality(query, requestedMuni string, hits []model.SearchHit, entities model.DetectedEntities, wards []model.WardRecord, officers []model.OfficerRecord) model.PromptedAnswer {
	a := model.PromptedAnswer{Query: query, Headline: "What the archive records", Confidence: "low"}
	seenFacts, seenLinks, seenContacts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	muni := strings.TrimSpace(requestedMuni)
	if muni == "" || muni == "all" {
		if len(hits) > 0 && hits[0].Municipality != "" {
			muni = hits[0].Municipality
		} else {
			muni = "manesar"
		}
	}
	fact := func(k, l, v string, i int) {
		key := k + ":" + strings.ToLower(v)
		if !seenFacts[key] {
			seenFacts[key] = true
			ef := model.ExtractedFact{Label: l, Kind: k, Value: v, CitationIndex: i}
			if k == "phone" {
				digits := strings.NewReplacer("-", "", " ", "").Replace(v)
				ef.Href = "tel:" + digits
			}
			a.Facts = append(a.Facts, ef)
		}
	}
	contact := func(c model.ContactInfo, i int) {
		key := strings.ToLower(c.Name + "|" + cmp.Or(c.Phone, c.Email, c.Role))
		if !seenContacts[key] && (c.Name != "" || c.Phone != "" || c.Email != "") {
			seenContacts[key] = true
			a.Contacts = append(a.Contacts, model.CitedContact{ContactInfo: c, CitationIndex: i})
		}
	}
	type point struct {
		text       string
		score, idx int
	}
	var points []point
	qt := terms(query)
	q := strings.ToLower(query)
	for j, h := range hits[:min(len(hits), 4)] {
		i := j + 1
		body := cmp.Or(h.MatchedChunkText, h.Snippet, h.Summary)
		a.Citations = append(a.Citations, model.PromptedCitation{Index: i, DocID: h.DocID, Municipality: h.Municipality, Title: h.Title, Category: h.Category, Heading: h.Heading, Excerpt: shorten(cleanMarkdownLine(cmp.Or(h.Snippet, body)), 340), SourceURL: h.SourceURL, Date: h.Date})
		for _, c := range h.Contacts {
			contact(c, i)
		}
		for _, raw := range append([]string{h.ActionURL}, linkPattern.FindAllString(body, -1)...) {
			if u, ok := official(raw); raw != "" && ok && !seenLinks[u] {
				seenLinks[u] = true
				parsed, _ := url.Parse(u)
				a.ActionLinks = append(a.ActionLinks, model.ActionLink{Label: parsed.Hostname(), URL: u, CitationIndex: i})
			}
		}
		for _, line := range sentences(body) {
			s := cleanMarkdownLine(line)
			if len([]rune(s)) < 18 {
				continue
			}
			score := 0
			if j == 0 {
				score += 3
			}
			ws := words.FindAllString(strings.ToLower(s), -1)
			for _, t := range qt {
				if slices.Contains(ws, t) {
					score += 4
				}
			}
			for _, f := range facts {
				if f.pattern.MatchString(s) {
					score += 2
				}
			}
			if strings.Contains(s, "@") || linkPattern.MatchString(line) {
				score++
			}
			points = append(points, point{s, score, i})
		}
	}
	slices.SortStableFunc(points, func(a, b point) int { return cmp.Compare(b.score, a.score) })
	for _, p := range points {
		if len(a.KeyPoints) == 5 {
			break
		}
		if slices.ContainsFunc(a.KeyPoints, func(k model.KeyPoint) bool { return k.Text == p.text }) {
			continue
		}
		a.KeyPoints = append(a.KeyPoints, model.KeyPoint{Text: p.text, CitationIndex: p.idx})
		for _, f := range facts {
			for _, v := range f.pattern.FindAllString(p.text, 3) {
				fact(f.kind, f.label, v, p.idx)
			}
		}
	}

	var structured []string
	directAnswer := false
	wantsDirectory := directoryCue.MatchString(query)
	roleMatches := func(role string) bool {
		role = strings.ToLower(role)
		for _, r := range []string{"junior engineer", "assistant engineer", "executive engineer", "councillor"} {
			if strings.Contains(q, r) {
				return strings.Contains(role, r)
			}
		}
		return strings.Contains(q, "engineer") && strings.Contains(role, "engineer")
	}
	citeFor := func(targetMuni, title, path, excerpt string) int {
		i := len(a.Citations) + 1
		a.Citations = append(a.Citations, model.PromptedCitation{
			Index:        i,
			Municipality: cmp.Or(targetMuni, muni),
			Title:        title,
			SourceURL:    path,
			Excerpt:      excerpt,
			Category:     "Directory",
		})
		return i
	}
	for _, w := range wards {
		if !slices.Contains(entities.Wards, w.WardNumber) {
			continue
		}
		isWhichWard := strings.Contains(q, "which ward") && (len(entities.Villages) > 0 || len(entities.Sectors) > 0)
		if !wantsDirectory && !isWhichWard {
			continue
		}
		rowMuni := cmp.Or(w.Municipality, muni)
		wardsURL := "/wards?municipality=" + url.QueryEscape(rowMuni)
		label := fmt.Sprintf("Ward %d", w.WardNumber)
		i := citeFor(rowMuni, label+" directory record", wardsURL, cmp.Or(w.BoundarySummary, w.Name, label))
		fact("ward", "Ward", label, i)
		parts := []string{label}
		if isWhichWard {
			directAnswer = true
		}
		if w.Zone != "" {
			parts = append(parts, "Zone: "+w.Zone)
		}
		if w.Division != "" {
			parts = append(parts, "Division: "+w.Division)
		}
		if w.Councillor != nil {
			c := model.ContactInfo{Name: w.Councillor.Name, Phone: w.Councillor.Phone, Role: "Councillor"}
			parts = append(parts, contactText(c))
			contact(c, i)
			directAnswer = directAnswer || c.Name != "" && roleMatches(c.Role)
		}
		for _, entry := range []struct {
			role string
			c    *model.ContactInfo
		}{{"Executive Engineer", w.ExecutiveEngineer}, {"Assistant Engineer", w.AssistantEngineer}} {
			if entry.c != nil {
				c := *entry.c
				c.Role = cmp.Or(c.Role, entry.role)
				parts = append(parts, contactText(c))
				contact(c, i)
				directAnswer = directAnswer || c.Name != "" && roleMatches(entry.role)
			}
		}
		jes := w.JuniorEngineers
		if len(jes) == 0 && w.JuniorEngineer != nil {
			jes = []model.ContactInfo{*w.JuniorEngineer}
		}
		for _, je := range jes {
			c := je
			c.Role = cmp.Or(c.Role, "Junior Engineer")
			parts = append(parts, contactText(c))
			contact(c, i)
			directAnswer = directAnswer || c.Name != "" && roleMatches("junior engineer")
		}
		record := strings.Join(parts, "; ")
		a.Citations[i-1].Excerpt = record
		structured = append(structured, record+fmt.Sprintf(". [%d]", i))
	}
	for _, o := range officers {
		role := strings.ToLower(o.Designation)
		inWard := slices.ContainsFunc(o.Wards, func(w int) bool { return slices.Contains(entities.Wards, w) })
		byName := o.Name != "" && strings.Contains(q, strings.ToLower(o.Name))
		byRole := false
		for _, r := range []string{"junior engineer", "assistant engineer", "executive engineer", "commissioner"} {
			if strings.Contains(q, r) && strings.Contains(role, r) {
				byRole = true
			}
		}
		if !byName && !(inWard && wantsDirectory && strings.Contains(role, "engineer")) && !(len(entities.Wards) == 0 && byRole) {
			continue
		}
		c := model.ContactInfo{Name: o.Name, Role: o.Designation, Phone: o.Phone, Email: o.Email}
		key := strings.ToLower(c.Name + "|" + cmp.Or(c.Phone, c.Email, c.Role))
		if seenContacts[key] {
			continue
		}
		t := contactText(c)
		rowMuni := cmp.Or(o.Municipality, muni)
		i := citeFor(rowMuni, "Officer directory: "+o.Name, "/wards?municipality="+url.QueryEscape(rowMuni), t)
		contact(c, i)
		directAnswer = directAnswer || byName || (inWard && roleMatches(c.Role))
		if byName || byRole || (inWard && wantsDirectory) {
			structured = append(structured, fmt.Sprintf("%s. [%d]", t, i))
		}
	}
	if len(structured) > 0 {
		a.AnswerParagraphs = append(a.AnswerParagraphs, strings.Join(structured, " "))
		a.Confidence = "medium"
		if directAnswer {
			a.Confidence = "high"
		}
	}
	for _, p := range a.KeyPoints {
		if len(a.AnswerParagraphs) >= 2 {
			break
		}
		a.AnswerParagraphs = append(a.AnswerParagraphs, fmt.Sprintf("%s [%d]", p.Text, p.CitationIndex))
	}
	if a.Confidence == "low" && len(points) > 0 && points[0].score >= 4 {
		a.Confidence = "medium"
	}
	if len(a.AnswerParagraphs) == 0 {
		a.Headline = "No matching evidence"
		a.AnswerParagraphs = []string{"The archive has no matching passage or directory record for this question."}
	}
	if len(entities.Wards) > 0 {
		w := entities.Wards[0]
		a.FollowUpQueries = append(a.FollowUpQueries, fmt.Sprintf("Tenders and works in Ward %d", w), fmt.Sprintf("Junior Engineer and Councillor for Ward %d", w), fmt.Sprintf("Villages and sectors in Ward %d", w))
	}
	if strings.Contains(q, "property") || strings.Contains(q, "tax") {
		a.FollowUpQueries = append(a.FollowUpQueries, "How to submit Property ID objection", "Village Lal Dora property list", "Property tax payment portal")
	}
	if len(a.FollowUpQueries) == 0 && len(hits) > 0 {
		h := hits[0]
		if h.Department != "" {
			a.FollowUpQueries = append(a.FollowUpQueries, h.Department+" contact details")
		}
		if h.Category != "" {
			a.FollowUpQueries = append(a.FollowUpQueries, h.Category+" notices and deadlines")
		}
		if h.Heading != "" {
			a.FollowUpQueries = append(a.FollowUpQueries, h.Heading+" application process")
		}
	}
	for _, s := range []string{"Where do I file a municipal complaint?", "Who is my ward engineer?", "What are the current tender deadlines?"} {
		if len(a.FollowUpQueries) >= 3 {
			break
		}
		a.FollowUpQueries = unique(a.FollowUpQueries, s)
	}
	a.FollowUpQueries = a.FollowUpQueries[:min(4, len(a.FollowUpQueries))]
	return a
}
