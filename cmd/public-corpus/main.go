package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/municipal-archives-india/manesar/internal/indexer"
	"github.com/municipal-archives-india/manesar/internal/model"
)

var directContact = regexp.MustCompile(`(?i)(mailto:|tel:|sms:|wa\.me/|(?:api|web)\.whatsapp\.com/|\S\s*(?:@|\[at\]|\(at\))\s*[\w-]+\s*(?:\.|\[dot\]|\(dot\))\s*[a-z]{2,}|(?:^|[^\d])(?:\+91[ .()-]*[6-9](?:[ .()-]*\d){9}|[6-9](?:[ .()-]*\d){9}|0(?:[ .()-]*\d){10}|(?:1800|1860)(?:[ .()-]*\d){6,7})(?:[^\d]|$))`)
var labeledNumber = regexp.MustCompile(`(?i)(?:mobile|mob\.|phone|telephone|helpline|helpdesk|whats[ \t]*app|contact[ \t]*(?:no\.?|number)|फोन|मोबाइल|दूरभाष)[^\d\r\n]{0,24}[+(\d][\d +().-]{5,}\d`)
var webURL = regexp.MustCompile(`https?://[^\s<>"\]]+`)

func contactLike(s string) bool {
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		if unicode.IsDigit(r) {
			for _, entry := range unicode.Digit.R16 {
				if r >= rune(entry.Lo) && r <= rune(entry.Hi) && (r-rune(entry.Lo))%rune(entry.Stride) == 0 {
					return '0' + ((r-rune(entry.Lo))/rune(entry.Stride))%10
				}
			}
			for _, entry := range unicode.Digit.R32 {
				if r >= rune(entry.Lo) && r <= rune(entry.Hi) && (r-rune(entry.Lo))%rune(entry.Stride) == 0 {
					return '0' + ((r-rune(entry.Lo))/rune(entry.Stride))%10
				}
			}
		}
		if unicode.IsSpace(r) && r != '\n' {
			return ' '
		}
		return r
	}, html.UnescapeString(s))
	return directContact.MatchString(s) || labeledNumber.MatchString(webURL.ReplaceAllString(s, " "))
}

func safeText(s string) string {
	if contactLike(s) {
		return ""
	}
	return s
}

func publicLink(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return safeText(s)
}

func publicBody(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if contactLike(line) {
			lines[i] = ""
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if contactLike(string(data)) {
		return fmt.Errorf("%s: contact-like text remains in structured fields", path)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func exportProfile(dst string, p model.MunicipalityProfile) error {
	p.HelpdeskPhone, p.TollFree, p.WhatsApp, p.Email = "", "", "", ""
	p.Headquarters, p.Description = safeText(p.Headquarters), safeText(p.Description)
	p.OfficialWebsite = publicLink(p.OfficialWebsite)
	return writeJSON(filepath.Join(dst, "municipality.json"), p)
}

func withoutContact(c *model.ContactInfo) {
	if c != nil {
		c.Phone, c.Email = "", ""
	}
}

func exportDirectory(src, dst string) error {
	wards, err := indexer.LoadMunicipalityWards(src)
	if err != nil {
		return err
	}
	for i := range wards {
		w := &wards[i]
		w.BoundarySummary, w.SourceURL = safeText(w.BoundarySummary), publicLink(w.SourceURL)
		if w.Councillor != nil {
			w.Councillor.Phone, w.Councillor.Notes = "", safeText(w.Councillor.Notes)
		}
		withoutContact(w.ExecutiveEngineer)
		withoutContact(w.AssistantEngineer)
		withoutContact(w.JuniorEngineer)
		for j := range w.JuniorEngineers {
			withoutContact(&w.JuniorEngineers[j])
		}
	}
	if len(wards) > 0 {
		if err := writeJSON(filepath.Join(dst, "wards.json"), wards); err != nil {
			return err
		}
	}
	officers, err := indexer.LoadMunicipalityOfficers(src)
	if err != nil {
		return err
	}
	for i := range officers {
		officers[i].Phone, officers[i].Email = "", ""
	}
	if len(officers) > 0 {
		return writeJSON(filepath.Join(dst, "officers.json"), officers)
	}
	return nil
}

func exportDocuments(root, src, dst, slug string) (count int, err error) {
	err = filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s: expected a regular record file", path)
		}
		doc, err := indexer.ParseMarkdownFile(path, root)
		if err != nil {
			return err
		}
		if doc.Municipality != slug {
			return fmt.Errorf("%s: municipality does not match directory", path)
		}
		doc.Contacts = nil
		doc.Summary, doc.OriginalFilename = safeText(doc.Summary), safeText(doc.OriginalFilename)
		doc.SourceURL, doc.ActionURL = publicLink(doc.SourceURL), publicLink(doc.ActionURL)
		body := publicBody(doc.Content)
		if doc.SourceURL == "" || (body == "" && doc.Summary == "") {
			return fmt.Errorf("%s: no safe official source or substantive text", path)
		}
		header, err := json.MarshalIndent(doc.DocumentFrontmatter, "", "  ")
		if err != nil {
			return err
		}
		if contactLike(string(header)) || contactLike(body) {
			return fmt.Errorf("%s: contact-like text remains", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, fmt.Appendf(nil, "---\n%s\n---\n%s\n", header, body), 0644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func export(root, dest string) (int, int, error) {
	profiles, err := indexer.ScanMunicipalities(root)
	if err != nil {
		return 0, 0, err
	}
	if err := os.Mkdir(dest, 0755); err != nil {
		return 0, 0, err
	}
	records := 0
	for _, p := range profiles {
		src, dst := filepath.Join(root, p.Slug), filepath.Join(dest, p.Slug)
		if err := os.Mkdir(dst, 0755); err != nil {
			return 0, 0, err
		}
		if err := exportProfile(dst, p); err != nil {
			return 0, 0, err
		}
		if err := exportDirectory(src, dst); err != nil {
			return 0, 0, err
		}
		n, err := exportDocuments(root, src, dst, p.Slug)
		if err != nil {
			return 0, 0, err
		}
		if n == 0 {
			return 0, 0, fmt.Errorf("%s: no public records", p.Slug)
		}
		records += n
	}
	return len(profiles), records, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: public-corpus ARCHIVES NEW_PUBLIC_ARCHIVES")
		os.Exit(2)
	}
	profiles, records, err := export(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Exported %d profiles and %d records without direct contact fields.\n", profiles, records)
}
