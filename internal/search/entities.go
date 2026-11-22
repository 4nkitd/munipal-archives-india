package search

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/municipal-archives-india/manesar/internal/model"
)

var wardPattern = regexp.MustCompile(`(?i)\b(?:ward(?:\s+no\.?)?|w)\s*[-#]?\s*([0-9]{1,3})\b`)
var sectorPattern = regexp.MustCompile(`(?i)\b(?:sector|sec)\s*[.-]?\s*([0-9]+[a-z]?)\b`)
var words = regexp.MustCompile(`[\pL\pN]+`)
var villages = strings.Split("Manesar,Kasan,Bhangrola,Hayatpur,Shikohpur,Naurangpur,Kankrola,Khoh,Badha,Sikanderpur Badha,Garhi Harsaru,Harsaru,Sihi,Dhorka,Wazirpur,Meoka,Nawada Fatehpur,Sehrawan,Baskushla,Basharia,Dhana,Fazalwas,Kukrola,Lakhnola,Nainwal,Jhund Sarai (Abad),Jhund Sarai (Viran),Bamroli,Naharpur Kasan,Bar Gujar,Rampura", ",")

var villageAliases = map[string]string{
	"bargujjar":         "Bar Gujar",
	"baskusla":          "Baskushla",
	"laknola":           "Lakhnola",
	"rampur":            "Rampura",
	"sikohpur":          "Shikohpur",
	"jhund sarai abad":  "Jhund Sarai (Abad)",
	"jhund sarai viran": "Jhund Sarai (Viran)",
}

var explicitManesarVillage = regexp.MustCompile(`(?i)\b(?:village\s+manesar|manesar\s+(?:village|gaon)|lal\s+dora\s+manesar|which\s+ward\s+(?:is|covers)\s+manesar)\b`)

func unique[T comparable](v []T, x T) []T {
	if !slices.Contains(v, x) {
		return append(v, x)
	}
	return v
}

func normalized(s string) string {
	return strings.Join(words.FindAllString(strings.ToLower(s), -1), " ")
}

func canonicalVillage(name string) string {
	if c, ok := villageAliases[normalized(name)]; ok {
		return c
	}
	return name
}

func sector(s string) string {
	if m := sectorPattern.FindStringSubmatch(s); m != nil {
		return strings.ToUpper(m[1])
	}
	return strings.ToUpper(strings.TrimSpace(s))
}

func DetectEntities(query string, wards []model.WardRecord) model.DetectedEntities {
	e := model.DetectedEntities{Wards: []int{}, Sectors: []string{}, Villages: []string{}}
	for _, m := range wardPattern.FindAllStringSubmatch(query, -1) {
		n, _ := strconv.Atoi(m[1])
		if n >= 1 && n <= 100 {
			e.Wards = unique(e.Wards, n)
		}
	}
	for _, m := range sectorPattern.FindAllStringSubmatch(query, -1) {
		e.Sectors = unique(e.Sectors, strings.ToUpper(m[1]))
	}
	names := []string{}
	for _, w := range wards {
		names = append(names, w.Villages...)
	}
	names = append(names, villages...)
	for alias := range villageAliases {
		names = append(names, alias)
	}
	slices.SortStableFunc(names, func(a, b string) int { return len(b) - len(a) })
	q := " " + normalized(query) + " "
	for _, n := range names {
		norm := normalized(n)
		// Avoid treating city-scope references ("in Manesar", "MC Manesar") as Manesar village.
		if norm == "manesar" && !explicitManesarVillage.MatchString(query) {
			continue
		}
		key := " " + norm + " "
		if key != "  " && strings.Contains(q, key) {
			canon := canonicalVillage(n)
			e.Villages = unique(e.Villages, canon)
			q = strings.ReplaceAll(q, key, " ")
		}
	}
	for _, w := range wards {
		for _, v := range w.Villages {
			vCanon := canonicalVillage(v)
			if slices.ContainsFunc(e.Villages, func(n string) bool { return strings.EqualFold(n, v) || strings.EqualFold(n, vCanon) }) {
				e.Wards = unique(e.Wards, w.WardNumber)
			}
		}
		for _, s := range w.Sectors {
			if slices.Contains(e.Sectors, sector(s)) {
				e.Wards = unique(e.Wards, w.WardNumber)
			}
		}
	}
	slices.Sort(e.Wards)
	return e
}
