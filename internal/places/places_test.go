package places

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		q, name, tz string
		kind        Kind
	}{
		{"berlin", "Berlin", "Europe/Berlin", KindCity},
		{"Rio de Janeiro", "Rio de Janeiro", "America/Sao_Paulo", KindCity},
		{"rio", "Rio de Janeiro", "America/Sao_Paulo", KindCity},
		{"salt lake city", "Salt Lake City", "America/Denver", KindCity},
		{"salt lake", "Salt Lake City", "America/Denver", KindCity},
		{"São Paulo", "São Paulo", "America/Sao_Paulo", KindCity},
		{"sao_paulo", "São Paulo", "America/Sao_Paulo", KindCity},
		{"zurich", "Zürich", "Europe/Zurich", KindCity},
		{"nyc", "New York City", "America/New_York", KindCity},
		{"new york", "New York City", "America/New_York", KindZone},
		{"bombay", "Mumbai", "Asia/Kolkata", KindCity},
		{"valencia, spain", "Valencia", "Europe/Madrid", KindCity},
		{"valencia es", "Valencia", "Europe/Madrid", KindCity},
		{"portland, me", "Portland", "America/New_York", KindCity},
		{"portland", "Portland", "America/Los_Angeles", KindCity},
		{"germany", "Berlin", "Europe/Berlin", KindCountry},
		{"DE", "Berlin", "Europe/Berlin", KindCountry},
		{"brazil", "Brasília", "America/Sao_Paulo", KindCountry},
		{"usa", "Washington", "America/New_York", KindCountry},
		{"uk", "London", "Europe/London", KindCountry},
		{"taiwan", "Taipei", "Asia/Taipei", KindCountry},
		{"Europe/Berlin", "Berlin", "Europe/Berlin", KindZone},
		{"america/los_angeles", "Los Angeles", "America/Los_Angeles", KindZone},
		{"Etc/GMT+3", "GMT+3", "Etc/GMT+3", KindZone},
		{"kolkata", "Kolkata", "Asia/Kolkata", KindCity},
		{"pst", "Los Angeles", "America/Los_Angeles", KindAbbr},
		{"MST", "Denver", "America/Denver", KindAbbr},
		{"pacific", "Los Angeles", "America/Los_Angeles", KindAbbr},
		{"cest", "Berlin", "Europe/Berlin", KindAbbr},
		{"ist", "Kolkata", "Asia/Kolkata", KindAbbr},
		{"utc", "UTC", "UTC", KindOffset},
		{"gmt", "GMT", "UTC", KindOffset},
		{"utc+5:30", "UTC+5:30", "UTC+05:30", KindOffset},
		{"GMT-3", "UTC-3", "UTC-03:00", KindOffset},
		{"+0545", "UTC+5:45", "UTC+05:45", KindOffset},
	}
	for _, tt := range tests {
		m, err := Resolve(tt.q)
		if err != nil {
			t.Errorf("Resolve(%q): %v", tt.q, err)
			continue
		}
		if m.Place.Name != tt.name || m.Place.TZ != tt.tz || m.Kind != tt.kind {
			t.Errorf("Resolve(%q) = %q %q kind %d, want %q %q kind %d",
				tt.q, m.Place.Name, m.Place.TZ, m.Kind, tt.name, tt.tz, tt.kind)
		}
		if _, err := m.Place.Location(); err != nil {
			t.Errorf("Resolve(%q): location: %v", tt.q, err)
		}
	}
}

func TestResolveSubtitles(t *testing.T) {
	for q, sub := range map[string]string{
		"salt lake city": "United States, Utah",
		"rio":            "Brazil",
		"taipei":         "Taiwan",
		"los angeles":    "United States, California",
	} {
		m, err := Resolve(q)
		if err != nil || m.Place.Sub != sub {
			t.Errorf("Resolve(%q).Sub = %q (%v), want %q", q, m.Place.Sub, err, sub)
		}
	}
}

func TestResolveNotes(t *testing.T) {
	m, _ := Resolve("valencia")
	if !strings.Contains(m.Note, "Spain") {
		t.Errorf("valencia note = %q, want a hint about Spain", m.Note)
	}
	m, _ = Resolve("brazil")
	if !strings.Contains(m.Note, "time zones") {
		t.Errorf("brazil note = %q, want a multi-zone hint", m.Note)
	}
	m, _ = Resolve("cst")
	if !strings.Contains(m.Note, "China") {
		t.Errorf("cst note = %q, want China hint", m.Note)
	}
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("mts")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("Resolve(mts) err = %v, want NotFoundError", err)
	}
	if len(nf.Suggestions) == 0 || !strings.HasPrefix(nf.Suggestions[0], "MST") {
		t.Errorf("suggestions for mts = %v, want MST first", nf.Suggestions)
	}
	for q, want := range map[string]string{"tokyp": "Tokyo", "frankfrut": "Frankfurt am Main", "lisboa": "Lisbon"} {
		_, err = Resolve(q)
		if !errors.As(err, &nf) || len(nf.Suggestions) == 0 || !strings.HasPrefix(nf.Suggestions[0], want) {
			t.Errorf("Resolve(%s) = %v, want %s suggested first", q, err, want)
		}
	}
}

func TestAbbrZoneAt(t *testing.T) {
	summer := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	winter := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		code string
		at   time.Time
		zone string
		ok   bool
	}{
		{"MST", winter, "America/Denver", true},
		{"MST", summer, "America/Phoenix", true},
		{"PST", winter, "America/Los_Angeles", true},
		{"PST", summer, "", false},
		{"PDT", summer, "America/Los_Angeles", true},
		{"PT", summer, "America/Los_Angeles", true},
		{"CET", summer, "", false},
		{"CEST", summer, "Europe/Berlin", true},
		{"SGT", summer, "Asia/Singapore", true},
	}
	for _, tt := range tests {
		a, _ := LookupAbbr(tt.code)
		zone, ok := a.ZoneAt(tt.at)
		if zone != tt.zone || ok != tt.ok {
			t.Errorf("%s.ZoneAt(%s) = %q %v, want %q %v", tt.code, tt.at.Month(), zone, ok, tt.zone, tt.ok)
		}
	}
}

func TestTag(t *testing.T) {
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for tz, want := range map[string]string{
		"Europe/Berlin":     "CEST",
		"America/Sao_Paulo": "BRT",
		"Asia/Taipei":       "CST",
		"Asia/Dubai":        "GST",
		"Asia/Kathmandu":    "NPT",
		"Asia/Tbilisi":      "UTC+4",
		"UTC+05:30":         "UTC+5:30",
	} {
		loc, err := LoadLocation(tz)
		if err != nil {
			t.Fatal(err)
		}
		if got := Tag(at, loc); got != want {
			t.Errorf("Tag(%s) = %q, want %q", tz, got, want)
		}
	}
}

func TestSearch(t *testing.T) {
	ms := Search("san", 8)
	if len(ms) < 5 {
		t.Fatalf("Search(san) = %d results, want several", len(ms))
	}
	ms = Search("berlin", 5)
	if ms[0].Place.TZ != "Europe/Berlin" {
		t.Errorf("Search(berlin)[0] = %+v", ms[0].Place)
	}
}

func TestPlaceForZone(t *testing.T) {
	for zone, name := range map[string]string{
		"Europe/Berlin":    "Berlin",
		"America/New_York": "New York City",
		"America/Denver":   "Denver",
		"Etc/UTC":          "UTC",
	} {
		if got := PlaceForZone(zone).Name; got != name {
			t.Errorf("PlaceForZone(%s) = %q, want %q", zone, got, name)
		}
	}
}
