package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/municipal-archives-india/manesar/internal/model"
)

const PublicContactsFilename = "public-contacts.json"

var publicNumber = regexp.MustCompile(`^\+?[0-9]{3,15}$`)
var publicEmail = regexp.MustCompile(`^[A-Za-z0-9_.'+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

func LoadPublicContacts(dir string) ([]model.PublicContact, error) {
	path := filepath.Join(dir, PublicContactsFilename)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var rows []model.PublicContact
	if err := decoder.Decode(&rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(rows) == 0 || decoder.Decode(new(json.RawMessage)) != io.EOF {
		return nil, fmt.Errorf("%s: expected one nonempty array", path)
	}
	type identity struct{ label, number, email string }
	seen := make(map[identity]bool, len(rows))
	for i, row := range rows {
		fail := func(field string) ([]model.PublicContact, error) {
			return nil, fmt.Errorf("%s: row %d: %s", path, i+1, field)
		}
		if row.Label == "" || strings.TrimSpace(row.Label) != row.Label || strings.ContainsFunc(row.Label, unicode.IsControl) {
			return fail("invalid label")
		}
		if row.Number == "" && row.Email == "" {
			return fail("expected a phone number or email")
		}
		if row.Number != "" && !publicNumber.MatchString(row.Number) {
			return fail("invalid dial string")
		}
		if row.Email != "" {
			addr, err := mail.ParseAddress(row.Email)
			if err != nil || addr.Address != row.Email || !publicEmail.MatchString(row.Email) {
				return fail("invalid email address")
			}
		}
		u, err := url.Parse(row.SourceURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.ContainsAny(row.SourceURL, " \t\r\n") {
			return fail("expected an HTTPS source URL")
		}
		key := identity{strings.ToLower(row.Label), row.Number, strings.ToLower(row.Email)}
		if seen[key] {
			return fail("duplicate public contact")
		}
		seen[key] = true
	}
	return rows, nil
}
