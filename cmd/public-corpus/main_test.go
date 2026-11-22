package main

import (
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
		"municipality.json": `{"slug":"demo","name":"Demo City","helpdeskPhone":"7000000000","email":"example@invalid.example","officialWebsite":"https://example.org"}`,
		"wards.json":        `[{"ward_number":1,"name":"First Ward","councillor":{"name":"Demo Councillor","phone":"7000000000"}}]`,
		"officers.json":     `[{"sr_no":1,"name":"Demo Officer","designation":"Junior Engineer","email":"example@invalid.example","wards":[1]}]`,
		"record.md": `---
id: demo-record
municipality: demo
title: Ward evidence
category: governance
doc_type: report
source_url: https://example.org/notices
contacts:
  - name: Demo Officer
    email: example@invalid.example
---
# Ward evidence

There are 15 territorial wards.
Contact the officer at example [at] invalid.example.
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
	doc, err := indexer.ParseMarkdownFile(filepath.Join(out, "record.md"), target)
	if err != nil || len(doc.Contacts) != 0 || !strings.Contains(doc.Content, "15 territorial wards") || strings.Contains(doc.Content, "[at]") {
		t.Fatalf("unexpected public record: %+v, %v", doc, err)
	}
	wards, err := indexer.LoadMunicipalityWards(out)
	if err != nil || len(wards) != 1 || wards[0].Councillor.Phone != "" || wards[0].JuniorEngineer.Email != "" {
		t.Fatalf("unexpected public wards: %+v, %v", wards, err)
	}
	if _, _, err := export(source, target); err == nil {
		t.Fatal("export overwrote its destination")
	}
}
