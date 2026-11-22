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

var directContact = regexp.MustCompile(`(?i)(mailto:|tel:|sms:|wa\.me/|(?:api|web)\.whatsapp\.com/|\S\s*(?:@|\[at\]|\(at\))\s*[\w-]+\s*(?:\.|\[dot\]|\(dot\))\s*[a-z]{2,})`)
var phoneNumber = regexp.MustCompile(`(?:^|[^\d])(?:\+91[ .()-]*[6-9](?:[ .()-]*\d){9}|[6-9](?:[ .()-]*\d){9}|0(?:[ .()-]*\d){10}|(?:1800|1860)(?:[ .()-]*\d){6,7})(?:[^\d]|$)`)
var labeledNumber = regexp.MustCompile(`(?i)(?:mobile|mob\.|phone|telephone|helpline|helpdesk|whats[ \t]*app|contact[ \t]*(?:no\.?|number)|फोन|मोबाइल|दूरभाष)[^\d\r\n]{0,24}[+(\d][\d +().-]{5,}\d`)
var webURL = regexp.MustCompile(`https?://[^\s<>"\]]+`)
var durgTenderPDF = regexp.MustCompile(`^https://municipalcorporationdurg\.in/Upload_Tender/[0-9]{20,24}@0\.pdf(?:#page=[0-9]+)?\b`)
var uuid = regexp.MustCompile(`(?i)\b[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\b`)
var phoneToken = regexp.MustCompile(`\d(?:[ \t().-]*\d){5,}`)
var encodedByte = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

func screenText(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	} else {
		s = encodedByte.ReplaceAllStringFunc(s, func(byteText string) string {
			value, _ := url.PathUnescape(byteText)
			return value
		})
	}
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		if unicode.In(r, unicode.Dash, unicode.Hyphen) {
			return '-'
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
}

func contactLike(s string) bool {
	s = screenText(s)
	s = webURL.ReplaceAllStringFunc(s, func(link string) string {
		return durgTenderPDF.ReplaceAllString(link, " ")
	})
	return directContact.MatchString(s) || phoneNumber.MatchString(uuid.ReplaceAllString(s, " ")) ||
		labeledNumber.MatchString(webURL.ReplaceAllString(s, " "))
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

func digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, screenText(s))
}

type contactSet struct {
	phones map[string]bool
	emails map[string]bool
}

func (c contactSet) add(phone, email string) {
	for _, token := range phoneToken.FindAllString(screenText(phone), -1) {
		number := digits(token)
		c.phones[number] = true
		if len(number) == 12 && strings.HasPrefix(number, "91") {
			c.phones[number[2:]] = true
		}
		if len(number) == 11 && strings.HasPrefix(number, "0") {
			c.phones[number[1:]] = true
		}
	}
	if email = strings.ToLower(strings.TrimSpace(screenText(email))); strings.Contains(email, "@") {
		c.emails[email] = true
	}
}

func (c contactSet) matches(s string) bool {
	if contactLike(s) {
		return true
	}
	text := strings.ToLower(screenText(s))
	for email := range c.emails {
		if strings.Contains(text, email) {
			return true
		}
	}
	return c.matchesPhone(webURL.ReplaceAllString(text, " "))
}

func (c contactSet) matchesPhone(text string) bool {
	for _, token := range phoneToken.FindAllString(text, -1) {
		if c.phones[digits(token)] {
			return true
		}
	}
	return false
}

func (c contactSet) matchesLink(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return true
	}
	query, err := url.QueryUnescape(u.RawQuery)
	if err != nil {
		query = u.RawQuery
	}
	return c.matches(link) || c.matchesPhone(screenText(u.Path)) ||
		c.matchesPhone(screenText(query)) || c.matchesPhone(screenText(u.Fragment))
}

func publicBody(body string, known contactSet) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if known.matches(line) {
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

func exportProfile(dst string, p model.MunicipalityProfile, known contactSet) error {
	if len(p.PublicContacts) != 0 {
		for i, row := range p.PublicContacts {
			if known.matches(row.Label) || publicLink(row.SourceURL) == "" || known.matchesLink(row.SourceURL) {
				return fmt.Errorf("%s: public contact row %d has contact-like text outside number and email", p.Slug, i+1)
			}
		}
		data, err := json.MarshalIndent(p.PublicContacts, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, indexer.PublicContactsFilename), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	p.PublicContacts = nil
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

func exportDirectory(src, dst string, known contactSet) error {
	wards, err := indexer.LoadMunicipalityWards(src)
	if err != nil {
		return err
	}
	unmerged, err := indexer.LoadMunicipalityWardRows(src)
	if err != nil {
		return err
	}
	for i := range wards {
		ward := &wards[i]
		w := &unmerged[i]
		w.Councillor = ward.Councillor
		w.BoundarySummary, w.SourceURL = safeText(w.BoundarySummary), publicLink(w.SourceURL)
		if w.Councillor != nil {
			known.add(w.Councillor.Phone, "")
			w.Councillor.Phone, w.Councillor.Notes = "", safeText(w.Councillor.Notes)
		}
		for _, contact := range []*model.ContactInfo{w.ExecutiveEngineer, w.AssistantEngineer, w.JuniorEngineer} {
			if contact != nil {
				known.add(contact.Phone, contact.Email)
				withoutContact(contact)
			}
		}
		for j := range w.JuniorEngineers {
			known.add(w.JuniorEngineers[j].Phone, w.JuniorEngineers[j].Email)
			withoutContact(&w.JuniorEngineers[j])
		}
	}
	if len(unmerged) > 0 {
		if err := writeJSON(filepath.Join(dst, "wards.json"), unmerged); err != nil {
			return err
		}
	}
	officers, err := indexer.LoadMunicipalityOfficers(src)
	if err != nil {
		return err
	}
	for i := range officers {
		known.add(officers[i].Phone, officers[i].Email)
		officers[i].Phone, officers[i].Email = "", ""
	}
	if len(officers) > 0 {
		return writeJSON(filepath.Join(dst, "officers.json"), officers)
	}
	return nil
}

func exportDocuments(root, src, dst, slug string, known contactSet) (count int, err error) {
	var docs []*model.ArchiveDocument
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
		for _, contact := range doc.Contacts {
			known.add(contact.Phone, contact.Email)
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, doc := range docs {
		path := doc.FilePath
		doc.Contacts = nil
		doc.Summary, doc.OriginalFilename = safeText(doc.Summary), safeText(doc.OriginalFilename)
		var tags []string
		for _, tag := range doc.Tags {
			if !known.matches(tag) {
				tags = append(tags, tag)
			}
		}
		doc.Tags = tags
		if known.matches(doc.Summary) {
			doc.Summary = ""
		}
		doc.SourceURL, doc.ActionURL = publicLink(doc.SourceURL), publicLink(doc.ActionURL)
		doc.Content = publicBody(doc.Content, known)
		if doc.SourceURL == "" || len(indexer.ChunkDocument(doc)) == 0 {
			return 0, fmt.Errorf("%s: no safe source link or searchable text", path)
		}
		header, err := json.MarshalIndent(doc.DocumentFrontmatter, "", "  ")
		if err != nil {
			return 0, err
		}
		if contactLike(string(header)) || known.matches(doc.Content) {
			return 0, fmt.Errorf("%s: contact-like text remains", path)
		}
		rel, err := filepath.Rel(src, filepath.Join(root, path))
		if err != nil {
			return 0, err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(target, fmt.Appendf(nil, "---\n%s\n---\n%s\n", header, doc.Content), 0644); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
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
		known := contactSet{phones: map[string]bool{}, emails: map[string]bool{}}
		known.add(p.HelpdeskPhone, p.Email)
		known.add(p.TollFree, "")
		known.add(p.WhatsApp, "")
		if err := os.Mkdir(dst, 0755); err != nil {
			return 0, 0, err
		}
		if err := exportDirectory(src, dst, known); err != nil {
			return 0, 0, err
		}
		n, err := exportDocuments(root, src, dst, p.Slug, known)
		if err != nil {
			return 0, 0, err
		}
		if n == 0 {
			return 0, 0, fmt.Errorf("%s: no public records", p.Slug)
		}
		if err := exportProfile(dst, p, known); err != nil {
			return 0, 0, err
		}
		records += n
	}
	return len(profiles), records, nil
}

func main() {
	if len(os.Args) == 4 && os.Args[1] == "audit" {
		if err := audit(os.Args[2], os.Args[3], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: public-corpus ARCHIVES NEW_PUBLIC_ARCHIVES\n       public-corpus audit PUBLIC_ARCHIVES DATA")
		os.Exit(2)
	}
	profiles, records, err := export(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Exported %d profiles and %d records, with approved contacts kept separately.\n", profiles, records)
}
