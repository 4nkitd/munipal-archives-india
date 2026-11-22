package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/municipal-archives-india/manesar/internal/model"
	"gopkg.in/yaml.v3"
)

func ParseMarkdownFile(path, archivesRoot string) (*model.ArchiveDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimPrefix(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\ufeff")
	lines := strings.Split(text, "\n")
	end := 1
	for end < len(lines) && strings.TrimRight(lines[end], " \t") != "---" {
		end++
	}
	if strings.TrimSpace(lines[0]) != "---" || end == len(lines) {
		return nil, fmt.Errorf("%s: missing YAML frontmatter delimiter", path)
	}
	var d model.ArchiveDocument
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &d.DocumentFrontmatter); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Municipality) == "" || strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Category) == "" || strings.TrimSpace(d.DocType) == "" {
		return nil, fmt.Errorf("%s: id, municipality, title, category and doc_type are required", path)
	}
	d.Wards = nonNil(d.Wards)
	d.Sectors = nonNil(d.Sectors)
	d.Villages = nonNil(d.Villages)
	d.Tags = nonNil(d.Tags)
	d.Contacts = nonNil(d.Contacts)
	rel, err := filepath.Rel(archivesRoot, path)
	if err != nil {
		return nil, err
	}
	d.FilePath = filepath.ToSlash(rel)
	d.Content = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	d.WordCount = len(strings.Fields(d.Content))
	return &d, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func ScanMunicipalities(archivesDir string) ([]model.MunicipalityProfile, error) {
	entries, err := os.ReadDir(archivesDir)
	if err != nil {
		return nil, err
	}
	out := []model.MunicipalityProfile{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(archivesDir, entry.Name(), "municipality.json")
		raw, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var p model.MunicipalityProfile
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if p.Slug != entry.Name() {
			return nil, fmt.Errorf("%s: slug %q does not match directory", path, p.Slug)
		}
		out = append(out, p)
	}
	return out, nil
}

func loadRecords[T any](dir, file, key string) ([]T, error) {
	path := filepath.Join(dir, file)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []T{}, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		var obj map[string]json.RawMessage
		if err = json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		raw = obj[key]
	}
	var records []T
	if err = json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return nonNil(records), nil
}

func LoadMunicipalityOfficers(muniDir string) ([]model.OfficerRecord, error) {
	return loadRecords[model.OfficerRecord](muniDir, "officers.json", "officers")
}

func LoadMunicipalityWards(muniDir string) ([]model.WardRecord, error) {
	wards, err := loadRecords[model.WardRecord](muniDir, "wards.json", "wards")
	if err != nil || len(wards) == 0 {
		return wards, err
	}
	council, err := loadRecords[struct {
		WardNumber int `json:"ward_number"`
		model.CouncillorInfo
	}](muniDir, "council.json", "councillors")
	if err != nil {
		return nil, err
	}
	officers, err := LoadMunicipalityOfficers(muniDir)
	if err != nil {
		return nil, err
	}
	for i := range wards {
		w := &wards[i]
		for _, c := range council {
			if w.WardNumber == c.WardNumber {
				w.Councillor = &c.CouncillorInfo
				break
			}
		}
		for _, o := range officers {
			if !slices.Contains(o.Wards, w.WardNumber) {
				continue
			}
			c := &model.ContactInfo{Name: o.Name, Role: o.Designation, Phone: o.Phone, Email: o.Email}
			role := strings.ToLower(strings.TrimSpace(o.Designation))
			switch {
			case strings.HasPrefix(role, "executive engineer"):
				w.ExecutiveEngineer = c
			case strings.HasPrefix(role, "assistant engineer"):
				w.AssistantEngineer = c
			case strings.HasPrefix(role, "junior engineer"):
				if w.JuniorEngineer == nil {
					w.JuniorEngineer = c
				}
				w.JuniorEngineers = append(w.JuniorEngineers, *c)
			}
		}
	}
	return wards, nil
}
