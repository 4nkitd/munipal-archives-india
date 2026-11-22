package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/municipal-archives-india/manesar/internal/indexer"
)

func TestContactScreen(t *testing.T) {
	for _, s := range []string{
		"Local desk: +91 (700) 000 0000",
		"मोबाइल: ७०००००००००",
		"020-2000-0000",
		"Telephone extension: 234-5678",
		"Contact: office &#64; example.org",
		"office [at] example [dot] org",
		"https://example.org/contact/office%40example.org",
		"100% and office%40example.org",
		"020–2000–0000",
		"[direct chat](https://web.whatsapp.com/send?phone=1234)",
	} {
		if !contactLike(s) {
			t.Errorf("did not detect %q", s)
		}
	}
	for _, s := range []string{
		"Budget signed on 2026-11-22.",
		"Read the public helpline directory.\n2026-11-22",
		"https://example.org/public/whatsapp-image-2026-06-15.jpeg",
		"https://sec.kerala.gov.in/public/ic/download/0da59b73-669c-4b42-9184-731185c8ef10",
		"https://municipalcorporationdurg.in/Upload_Tender/100420251744266686783@0.pdf#page=2",
	} {
		if contactLike(s) {
			t.Errorf("incorrectly detected %q", s)
		}
	}
}

func TestExport(t *testing.T) {
	base := t.TempDir()
	source, target := filepath.Join(base, "archives"), filepath.Join(base, "public")
	city := filepath.Join(source, "demo")
	if err := os.MkdirAll(city, 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"municipality.json":    `{"slug":"demo","name":"Demo City","helpdeskPhone":"02020000000","email":"example@invalid.example","officialWebsite":"https://example.org"}`,
		"public-contacts.json": `[{"label":"Civic & service desk","number":"02020000000","email":"civic@example.org","source_url":"https://example.org/contact?view=public&lang=en"}]`,
		"wards.json":           `[{"ward_number":1,"name":"First Ward","councillor":{"name":"Demo Councillor","phone":"7000000000"}}]`,
		"officers.json":        `[{"sr_no":1,"name":"Demo Officer","designation":"Junior Engineer","email":"example@invalid.example","phone":"2345678","wards":[1]}]`,
		"record.md": `---
id: demo-record
municipality: demo
title: Ward evidence
category: governance
doc_type: report
source_url: https://example.org/notices
tags: ["02020000000", "ward-record"]
contacts:
  - name: Demo Officer
    email: example@invalid.example
---
# Ward evidence

There are 15 territorial wards.
Contact the officer at example [at] invalid.example.
Demo Officer | 234-5678
Local desk | 2020000000
`,
	} {
		if err := os.WriteFile(filepath.Join(city, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	profiles, records, err := export(source, target)
	if err != nil || profiles != 1 || records != 1 {
		t.Fatalf("export: %d profiles, %d records, %v", profiles, records, err)
	}
	out := filepath.Join(target, "demo")
	approved, err := indexer.ScanMunicipalities(target)
	if err != nil || len(approved) != 1 || approved[0].HelpdeskPhone != "" || approved[0].Email != "" ||
		len(approved[0].PublicContacts) != 1 || approved[0].PublicContacts[0].Number != "02020000000" ||
		approved[0].PublicContacts[0].Email != "civic@example.org" {
		t.Fatalf("explicit contact not preserved or legacy contact leaked: %v", err)
	}
	doc, err := indexer.ParseMarkdownFile(filepath.Join(out, "record.md"), target)
	if err != nil || len(doc.Contacts) != 0 || !strings.Contains(doc.Content, "15 territorial wards") ||
		strings.Contains(doc.Content, "[at]") || strings.Contains(doc.Content, "234-5678") || strings.Contains(doc.Content, "2020000000") ||
		len(doc.Tags) != 1 || doc.Tags[0] != "ward-record" {
		t.Fatalf("unexpected public record: %+v, %v", doc, err)
	}
	wards, err := indexer.LoadMunicipalityWards(out)
	if err != nil || len(wards) != 1 || wards[0].Councillor.Phone != "" || wards[0].JuniorEngineer.Email != "" || len(wards[0].JuniorEngineers) != 1 {
		t.Fatalf("unexpected public wards: %+v, %v", wards, err)
	}
	if _, _, err := export(source, target); err == nil {
		t.Fatal("export overwrote its destination")
	}
	for i, row := range []string{
		`[{"label":"Wrong label 234-5678","number":"01234567890","source_url":"https://example.org/contact"}]`,
		`[{"label":"Desk","number":"01234567890","source_url":"https://example.org/contact/234%2D5678"}]`,
	} {
		if err := os.WriteFile(filepath.Join(city, indexer.PublicContactsFilename), []byte(row), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := export(source, filepath.Join(base, fmt.Sprintf("bad-%d", i))); err == nil {
			t.Fatal("export accepted legacy contact data outside the approved fields")
		}
	}
}

func TestAuditPublicContacts(t *testing.T) {
	root := t.TempDir()
	city := filepath.Join(root, "demo")
	if err := os.Mkdir(city, 0755); err != nil {
		t.Fatal(err)
	}
	rows := `[{"label":"Civic desk","number":"01234567890","email":"desk@example.org","source_url":"https://example.org/contact?dept=civic&lang=en"}]`
	for name, data := range map[string]string{"municipality.json": `{"slug":"demo"}`, indexer.PublicContactsFilename: rows} {
		if err := os.WriteFile(filepath.Join(city, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	db, err := indexer.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	doubleEncoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fields string
		ok     bool
	}{
		{`"public_contacts":` + strings.ReplaceAll(rows, "&", `\u0026`), true},
		{`"public_contacts":` + strings.ReplaceAll(rows, "desk@example.org", "unapproved@example.org"), false},
		{`"public_contacts":` + rows + `,"PUBLIC_CONTACTS":[]`, false},
		{`"public_contacts":` + string(doubleEncoded), false},
		{`"public_contacts":[]`, false},
	} {
		if _, err := db.Exec(`INSERT OR REPLACE INTO municipalities VALUES ('demo', ?)`, `{"slug":"demo",`+tc.fields+`}`); err != nil {
			t.Fatal(err)
		}
		if err := auditPublicContacts(context.Background(), db, root); (err == nil) != tc.ok {
			t.Fatalf("audit acceptance = %v, want %v (%v)", err == nil, tc.ok, err)
		}
	}
}
