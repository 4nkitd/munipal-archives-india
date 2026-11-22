package server

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/municipal-archives-india/manesar/internal/model"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

var registerWord = regexp.MustCompile(`[\pL\pM\pN]+`)
var registerNumber = regexp.MustCompile(`^[0-9]+$`)

func registerWords(text string) []string {
	return registerWord.FindAllString(norm.NFC.String(strings.ToLower(text)), -1)
}

func registerTerms(d model.ArchiveDocument) string {
	return strings.Join(registerWords(strings.NewReplacer("-", " ", "_", " ").Replace(d.Category)+" "+d.Title+" "+d.DocType+" "+d.Municipality+" "+strings.Join(d.Tags, " ")), " ")
}

func matchesRegister(d model.ArchiveDocument, category string, terms []string) bool {
	if category != "" && d.Category != category {
		return false
	}
	if len(terms) == 0 {
		return true
	}
	words := strings.Fields(registerTerms(d))
	for _, term := range terms {
		numeric := registerNumber.MatchString(term)
		if !slices.ContainsFunc(words, func(word string) bool {
			return (numeric && word == term) || (!numeric && strings.Contains(word, term))
		}) {
			return false
		}
	}
	return true
}

func municipalityGroups(profiles []model.MunicipalityProfile) []model.MunicipalityGroup {
	states := make(map[string][]model.MunicipalityProfile)
	for _, p := range profiles {
		state := strings.TrimSpace(p.State)
		if state == "" {
			state = "Other municipalities"
		}
		states[state] = append(states[state], p)
	}
	names := make([]string, 0, len(states))
	for state := range states {
		names = append(names, state)
	}
	sort.Strings(names)
	groups := make([]model.MunicipalityGroup, 0, len(names))
	for _, state := range names {
		groups = append(groups, model.MunicipalityGroup{State: state, Profiles: states[state]})
	}
	return groups
}

func municipalityLabel(slug string) string {
	switch slug {
	case "all":
		return "All municipalities"
	case "anand":
		return "Karamsad Anand"
	case "navsari-vijalpore":
		return "Navsari"
	case "haldwani-kathgodam":
		return "Haldwani-Kathgodam"
	case "mathura-vrindavan":
		return "Mathura-Vrindavan"
	case "yamunanagar":
		return "Yamunanagar-Jagadhri"
	case "srinagar":
		return "Srinagar, Jammu and Kashmir"
	case "srinagar-uttarakhand":
		return "Srinagar Garhwal"
	default:
		return cases.Title(language.English).String(strings.ReplaceAll(slug, "-", " "))
	}
}

func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := s.pageData(name, r)
		var body bytes.Buffer
		if err == nil {
			err = s.tmpl.ExecuteTemplate(&body, name+".html", data)
		}
		if err != nil {
			code, msg := publicError(err)
			http.Error(w, msg, code)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = body.WriteTo(w)
	}
}

func (s *Server) pageData(name string, r *http.Request) (*model.PageData, error) {
	req, err := parseRequest(r)
	if err != nil {
		return nil, err
	}
	switch name {
	case "home", "search":
		req.Sort, err = searchSort(req.Sort)
	case "index":
		req.Sort, err = indexSort(req.Sort)
	case "document":
		req.Municipality = r.PathValue("municipality")
	}
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	profiles, err := s.svc.ListMunicipalities(ctx)
	if err != nil {
		return nil, err
	}
	data := &model.PageData{
		ActiveNav: name, MunicipalityCount: len(profiles), MunicipalityGroups: municipalityGroups(profiles),
		Query: req.Query, Mode: req.Mode, Sort: req.Sort,
		SelectedCat: req.Category, SelectedDocType: req.DocType, SelectedWard: req.Ward,
	}
	if req.Municipality == "all" {
		data.Municipality = model.MunicipalityProfile{
			Slug:      "all",
			Name:      "All indexed municipalities",
			ShortName: "All municipalities",
			Status:    "indexed",
		}
	} else {
		for _, p := range profiles {
			if p.Slug == req.Municipality {
				data.Municipality = p
				break
			}
		}
	}
	if data.Municipality.Slug == "" {
		return nil, fs.ErrNotExist
	}
	data.MunicipalityLabel = municipalityLabel(data.Municipality.Slug)
	globalStats, err := s.svc.Stats(ctx)
	if err != nil {
		return nil, err
	}
	if globalStats == nil {
		return nil, fmt.Errorf("archive statistics are missing")
	}
	// Scope municipal stats and category counts to the selected corporation (or global when "all").
	scopedStats := *globalStats
	if req.Municipality != "all" {
		muniDocs, err := s.svc.ListDocuments(ctx, req.Municipality, "", "", 0)
		if err != nil {
			return nil, err
		}
		muniWards, err := s.svc.ListWards(ctx, req.Municipality)
		if err != nil {
			return nil, err
		}
		muniOfficers, err := s.svc.ListOfficers(ctx, req.Municipality)
		if err != nil {
			return nil, err
		}
		catMap := map[string]int{}
		chunks := 0
		words := 0
		for _, d := range muniDocs {
			catMap[d.Category]++
			chunks += d.ChunkCount
			words += d.WordCount
		}
		cats := make([]model.CategoryCount, 0, len(catMap))
		for k, v := range catMap {
			cats = append(cats, model.CategoryCount{Category: k, Count: v})
		}
		sort.Slice(cats, func(i, j int) bool {
			if cats[i].Count != cats[j].Count {
				return cats[i].Count > cats[j].Count
			}
			return cats[i].Category < cats[j].Category
		})
		scopedStats.TotalDocuments = len(muniDocs)
		scopedStats.TotalChunks = chunks
		scopedStats.TotalWords = words
		scopedStats.WardsMapped = len(muniWards)
		scopedStats.OfficersMapped = len(muniOfficers)
		scopedStats.Categories = cats
	}
	data.Stats = &scopedStats
	data.Categories = scopedStats.Categories

	switch req.Municipality {
	case "manesar":
		data.PromptExamples = []string{
			"Which ward covers Bhangrola?",
			"Who is the Junior Engineer for Ward 9?",
			"How do I verify Lal Dora Property ID and submit objection?",
			"Bhangrola pond rejuvenation tender cost and deadline",
			"What restrictions apply under GRAP Stage 3?",
		}
	case "gurugram":
		data.PromptExamples = []string{
			"Who is the Mayor and Commissioner of Gurugram?",
			"Property tax payment and self-certification in Gurugram",
			"Bandhwari solid waste management plant and sanitation contracts",
			"MCG vs GMDA responsibility for roads, water and drainage",
			"Regularized colonies and ward boundaries in Gurugram",
		}
	case "faridabad":
		data.PromptExamples = []string{
			"Zones and ward division in Municipal Corporation Faridabad",
			"Property tax assessment and grievance helpline in Faridabad",
			"MCF vs FMDA jurisdiction for water supply and roads",
			"Solid waste management and Smart City works in Faridabad",
		}
	case "all":
		data.PromptExamples = []string{
			"Which municipal portals handle property tax and assessments?",
			"Who is the Junior Engineer for Ward 9 in Manesar?",
			"Bandhwari waste plant serving Gurugram and Faridabad",
			"GRAP Stage 3 construction restrictions in NCR",
			"Right to Service timelines for birth and marriage certificates",
		}
	default:
		city := data.MunicipalityLabel
		data.PromptExamples = []string{
			"Wards and elected council of " + city,
			"Officers directory and grievance helpline for " + city,
			"Property tax and citizen services in " + city,
			"Municipal tenders, budget and infrastructure works in " + city,
		}
	}

	switch name {
	case "home":
		data.PageTitle = "Search the civic record of " + data.MunicipalityLabel
	case "search":
		data.PageTitle = "Search the archive"
		if req.Query != "" {
			data.PageTitle = "Search: " + req.Query
			data.AskResult, err = s.svc.Ask(ctx, req)
		}
	case "document":
		data.Document, data.RenderedHTML, err = s.readDocument(r)
		if err == nil {
			data.PageTitle = data.Document.Title
		}
	case "wards":
		data.PageTitle = "Wards and officers"
		data.Wards, err = s.svc.ListWards(ctx, req.Municipality)
		if err == nil {
			data.Officers, err = s.svc.ListOfficers(ctx, req.Municipality)
		}
	case "index":
		data.PageTitle = "Archive index"
		var docs []model.ArchiveDocument
		docs, err = s.svc.ListDocumentsSorted(ctx, req.Municipality, "", req.DocType, req.Ward, req.Sort)
		terms := registerWords(req.Query)
		for _, d := range docs {
			if matchesRegister(d, req.Category, terms) {
				data.Documents = append(data.Documents, d)
			} else {
				data.OtherDocuments = append(data.OtherDocuments, d)
			}
		}
	}
	return data, err
}
