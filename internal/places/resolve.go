package places

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Kind says how a query was understood.
type Kind int

const (
	KindCity Kind = iota
	KindCountry
	KindAbbr
	KindOffset
	KindZone
)

// Match is a resolved query.
type Match struct {
	Place Place
	Kind  Kind
	Abbr  *Abbr  // set for KindAbbr
	Note  string // ambiguity hint worth showing to the user
}

// NotFoundError is returned when nothing matches; it carries "did you mean"
// suggestions.
type NotFoundError struct {
	Query       string
	Suggestions []string
}

func (e *NotFoundError) Error() string {
	msg := fmt.Sprintf("no place or time zone matches %q", e.Query)
	if len(e.Suggestions) > 0 {
		msg += "; did you mean " + strings.Join(e.Suggestions, ", ") + "?"
	}
	return msg
}

// Common short names that GeoNames doesn't carry as alternates.
var cityAliases = map[string]string{
	"nyc": "new york city", "ny": "new york city", "la": "los angeles",
	"sf": "san francisco", "dc": "washington, us", "hk": "hong kong",
	"sp": "sao paulo", "rio": "rio de janeiro", "bh": "belo horizonte",
	"cdmx": "mexico city", "kl": "kuala lumpur", "slc": "salt lake city",
}

// Resolve turns a query into a single best match.
func Resolve(q string) (Match, error) {
	if m, ok := resolve(q); ok {
		return m, nil
	}
	return Match{}, &NotFoundError{Query: strings.TrimSpace(q), Suggestions: Suggest(q, 3)}
}

func resolve(q string) (Match, bool) {
	raw := strings.TrimSpace(q)
	n := Normalize(raw)
	if n == "" {
		return Match{}, false
	}
	d := db()

	if off, ok := ParseOffset(n); ok {
		return offsetMatch(n, off), true
	}
	if strings.Contains(raw, "/") {
		return zoneMatch(raw)
	}
	if a, ok := LookupAbbr(n); ok {
		return abbrMatch(a), true
	}
	if target, ok := cityAliases[n]; ok {
		return resolve(target)
	}
	if i, ok := d.countryBy[n]; ok {
		return countryMatch(d.countries[i]), true
	}
	if idx := d.byName[n]; len(idx) > 0 {
		return cityMatch(idx), true
	}
	if m, ok := qualifiedCity(n); ok {
		return m, true
	}
	for lower, zone := range d.zones {
		if Normalize(filepath.Base(lower)) == n {
			return Match{Place: PlaceForZone(zone), Kind: KindZone}, true
		}
	}
	if idx := d.byAlt[n]; len(idx) > 0 {
		return cityMatch(idx), true
	}
	// "rio de" or "frankf": a prefix that ends on a word boundary, or is long
	// enough not to be noise ("mts" must not silently become Mtsensk).
	for i, name := range d.normNames {
		if strings.HasPrefix(name, n) && (len(n) >= 4 || len(name) == len(n) || name[len(n)] == ' ') {
			return Match{Place: d.cityPlace(d.cities[i]), Kind: KindCity}, true
		}
	}
	return Match{}, false
}

func offsetMatch(n string, off int) Match {
	p := Place{Name: FormatOffset(off), Sub: "fixed offset", TZ: OffsetTZ(off)}
	switch {
	case off == 0 && n == "gmt":
		p.Name, p.Sub = "GMT", "Greenwich Mean Time"
	case off == 0:
		p.Sub = "Coordinated Universal Time"
	}
	return Match{Place: p, Kind: KindOffset}
}

func zoneMatch(raw string) (Match, bool) {
	if zone, ok := db().zones[strings.ToLower(raw)]; ok {
		return Match{Place: PlaceForZone(zone), Kind: KindZone}, true
	}
	// Valid tzdata names that no city uses, like "Etc/GMT+3" or "US/Pacific".
	if _, err := time.LoadLocation(raw); err == nil {
		return Match{Place: Place{Name: strings.ReplaceAll(filepath.Base(raw), "_", " "), Sub: raw, TZ: raw}, Kind: KindZone}, true
	}
	return Match{}, false
}

func abbrMatch(a Abbr) Match {
	m := Match{Place: PlaceForZone(a.Zones[0]), Kind: KindAbbr, Abbr: &a}
	if a.Alt != "" {
		m.Note = fmt.Sprintf("%s is %s; %s", a.Code, a.Name, a.Alt)
	}
	return m
}

func countryMatch(k Country) Match {
	d := db()
	best := -1
	for _, i := range d.byName[Normalize(k.Capital)] {
		if d.cities[i].CC == k.ISO2 {
			best = i
			break
		}
	}
	if best < 0 {
		for i, c := range d.cities {
			if c.CC == k.ISO2 {
				best = i
				break
			}
		}
	}
	if best < 0 {
		return Match{Place: Place{Name: k.Name, TZ: "UTC"}, Kind: KindCountry}
	}
	c := d.cities[best]
	m := Match{Place: d.cityPlace(c), Kind: KindCountry}
	m.Place.Sub = k.Name
	if n := countryOffsets(k.ISO2); n > 1 {
		m.Note = fmt.Sprintf("%s spans %d time zones; using %s. Name a city for the others.", k.Name, n, c.Name)
	}
	return m
}

func countryOffsets(cc string) int {
	now := time.Now()
	seen := map[int]bool{}
	tzs := map[string]bool{}
	for _, c := range db().cities {
		if c.CC != cc || tzs[c.TZ] {
			continue
		}
		tzs[c.TZ] = true
		if loc, err := LoadLocation(c.TZ); err == nil {
			_, off := now.In(loc).Zone()
			seen[off] = true
		}
	}
	return len(seen)
}

// cityMatch picks the biggest city and mentions a namesake elsewhere when it
// is big enough to be what the user meant (Valencia, Venezuela vs Spain).
func cityMatch(idx []int) Match {
	d := db()
	c := d.cities[idx[0]]
	m := Match{Place: d.cityPlace(c), Kind: KindCity}
	now := time.Now()
	_, off := now.In(mustLoc(c.TZ)).Zone()
	for _, i := range idx[1:] {
		o := d.cities[i]
		if o.Pop*5 < c.Pop {
			break
		}
		if _, ooff := now.In(mustLoc(o.TZ)).Zone(); ooff != off {
			k, _ := d.country(o.CC)
			m.Note = fmt.Sprintf("using %s, %s; for %s, %s use '%s, %s'",
				c.Name, m.Place.Sub, o.Name, k.Name, strings.ToLower(o.ASCII), strings.ToLower(k.Name))
			break
		}
	}
	return m
}

// qualifiedCity handles "valencia, spain", "portland oregon" and "paris tx".
func qualifiedCity(n string) (Match, bool) {
	try := func(name, qual string) (Match, bool) {
		name, qual = strings.TrimSpace(name), strings.TrimSpace(qual)
		if name == "" || qual == "" {
			return Match{}, false
		}
		d := db()
		if target, ok := cityAliases[name]; ok {
			name = target
		}
		for _, i := range d.byName[name] {
			if d.qualifies(d.cities[i], qual) {
				return Match{Place: d.cityPlace(d.cities[i]), Kind: KindCity}, true
			}
		}
		return Match{}, false
	}
	if i := strings.LastIndex(n, ","); i > 0 {
		return try(n[:i], n[i+1:])
	}
	words := strings.Fields(n)
	for k := 1; k <= 2 && k < len(words); k++ {
		if m, ok := try(strings.Join(words[:len(words)-k], " "), strings.Join(words[len(words)-k:], " ")); ok {
			return m, true
		}
	}
	return Match{}, false
}

func (d *database) qualifies(c City, qual string) bool {
	if k, ok := d.countryBy[qual]; ok && d.countries[k].ISO2 == c.CC {
		return true
	}
	if c.CC == "US" && strings.EqualFold(c.AdminCode, qual) {
		return true
	}
	admin := Normalize(c.Admin)
	return admin == qual || (len(qual) >= 3 && strings.HasPrefix(admin, qual))
}

// PlaceForZone describes an IANA zone by its namesake city, falling back to
// the biggest city in the zone ("America/New_York" -> New York City).
func PlaceForZone(zone string) Place {
	switch zone {
	case "UTC", "Etc/UTC", "Etc/UCT", "UCT", "Zulu", "Etc/Zulu", "GMT", "Etc/GMT":
		return Place{Name: "UTC", Sub: "Coordinated Universal Time", TZ: "UTC"}
	}
	d := db()
	base := Normalize(filepath.Base(zone))
	for _, i := range d.byName[base] {
		if d.cities[i].TZ == zone {
			return d.cityPlace(d.cities[i])
		}
	}
	for _, c := range d.cities {
		if c.TZ == zone {
			return d.cityPlace(c)
		}
	}
	return Place{Name: strings.ReplaceAll(filepath.Base(zone), "_", " "), Sub: zone, TZ: zone}
}

// LocalZone is the system's IANA zone name, from $TZ or /etc/localtime.
func LocalZone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(target, "zoneinfo/"); i >= 0 {
			return target[i+len("zoneinfo/"):]
		}
	}
	return "UTC"
}

// Search returns up to limit candidates, best first. The first one is what
// Resolve would pick, when Resolve succeeds.
func Search(q string, limit int) []Match {
	n := Normalize(q)
	if n == "" {
		return nil
	}
	d := db()
	var out []Match
	seen := map[string]bool{}
	add := func(m Match) bool {
		key := m.Place.Name + "|" + m.Place.Sub + "|" + m.Place.TZ
		if !seen[key] {
			seen[key] = true
			out = append(out, m)
		}
		return len(out) >= limit
	}

	if m, ok := resolve(q); ok && add(m) {
		return out
	}
	for _, a := range abbrs {
		if len(n) >= 2 && strings.HasPrefix(strings.ToLower(a.Code), n) && add(abbrMatch(a)) {
			return out
		}
	}
	for _, k := range d.countries {
		if len(n) >= 3 && strings.HasPrefix(Normalize(k.Name), n) && add(countryMatch(k)) {
			return out
		}
	}
	for _, i := range d.byName[n] {
		if add(Match{Place: d.cityPlace(d.cities[i]), Kind: KindCity}) {
			return out
		}
	}
	for i, name := range d.normNames {
		if strings.HasPrefix(name, n) && add(Match{Place: d.cityPlace(d.cities[i]), Kind: KindCity}) {
			return out
		}
	}
	var zones []string
	for lower, zone := range d.zones {
		if strings.Contains(Normalize(lower), n) {
			zones = append(zones, zone)
		}
	}
	sort.Strings(zones)
	for _, z := range zones {
		if add(Match{Place: PlaceForZone(z), Kind: KindZone}) {
			return out
		}
	}
	return out
}

// Suggest offers near misses for typos like "mts" (MST) or "tokio".
func Suggest(q string, limit int) []string {
	n := Normalize(q)
	if n == "" {
		return nil
	}
	maxDist := 1
	if len(n) > 4 {
		maxDist = 2
	}
	type cand struct {
		label      string
		dist, skew int // edit distance, length difference
		rank       int // lower is better: abbreviations, then countries, then by population
	}
	var cands []cand
	consider := func(label, key string, rank int) {
		if abs(len(key)-len(n)) > maxDist {
			return
		}
		if dist := osa(n, key); dist <= maxDist {
			cands = append(cands, cand{label, dist, abs(len(key) - len(n)), rank})
		}
	}
	for _, a := range abbrs {
		consider(fmt.Sprintf("%s (%s)", a.Code, a.Name), strings.ToLower(a.Code), 0)
	}
	d := db()
	for _, k := range d.countries {
		consider(k.Name, Normalize(k.Name), 1)
	}
	// Compare against as many leading words as the query has, so "frankfrut"
	// still finds "Frankfurt am Main".
	words := len(strings.Fields(n))
	for i, name := range d.normNames {
		c := d.cities[i]
		if c.Pop < 100_000 {
			continue
		}
		if f := strings.Fields(name); len(f) > words {
			name = strings.Join(f[:words], " ")
		}
		k, _ := d.country(c.CC)
		consider(fmt.Sprintf("%s (%s)", c.Name, k.Name), name, 2+i)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		if a.skew != b.skew {
			return a.skew < b.skew
		}
		return a.rank < b.rank
	})
	var out []string
	for _, c := range cands {
		if len(out) == limit {
			break
		}
		out = append(out, c.label)
	}
	return out
}

// osa is the optimal string alignment distance: Levenshtein plus adjacent
// transpositions, so "mts" is one edit from "mst".
func osa(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ObservesDST reports whether the zone changes its offset during the year.
func ObservesDST(loc *time.Location, year int) bool {
	_, jan := time.Date(year, 1, 1, 12, 0, 0, 0, loc).Zone()
	_, jul := time.Date(year, 7, 1, 12, 0, 0, 0, loc).Zone()
	return jan != jul
}

func mustLoc(tz string) *time.Location {
	loc, err := LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Lookup is Resolve without the "did you mean" work, for callers that probe
// several readings of the input.
func Lookup(q string) (Match, bool) { return resolve(q) }
