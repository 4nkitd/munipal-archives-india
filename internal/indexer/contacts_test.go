package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPublicContacts(t *testing.T) {
	dir := t.TempDir()
	if rows, err := LoadPublicContacts(dir); err != nil || len(rows) != 0 {
		t.Fatalf("optional directory: %v, %v", rows, err)
	}
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"phone", `[{"label":"Ward 8","number":"+911234567890","source_url":"https://example.org/wards"}]`, true},
		{"email", `[{"label":"Office","email":"office@city.example","source_url":"https://example.org/office"}]`, true},
		{"no value", `[{"label":"Office","source_url":"https://example.org"}]`, false},
		{"not dialable", `[{"label":"Office","number":"123;ext=456","source_url":"https://example.org"}]`, false},
		{"mail headers", `[{"label":"Office","email":"office@city.example?bcc=a@b.example","source_url":"https://example.org"}]`, false},
		{"source scheme", `[{"label":"Office","number":"155313","source_url":"javascript:alert(1)"}]`, false},
		{"source userinfo", `[{"label":"Office","number":"155313","source_url":"https://office@example.org"}]`, false},
		{"unknown field", `[{"label":"Office","number":"155313","source_url":"https://example.org","phone":"101"}]`, false},
		{"duplicates", `[{"label":"Office","number":"123","source_url":"https://example.org"},{"label":"office","number":"123","source_url":"https://example.org/2"}]`, false},
		{"empty", `[]`, false},
		{"trailing", `[{"label":"Office","number":"155313","source_url":"https://example.org"}] {}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, PublicContactsFilename), []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPublicContacts(dir); (err == nil) != tc.ok {
				t.Fatalf("acceptance = %v, want %v (%v)", err == nil, tc.ok, err)
			}
		})
	}
}
