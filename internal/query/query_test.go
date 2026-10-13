package query

import (
	"strings"
	"testing"
	"time"

	"github.com/medeirosthiago/wtd/internal/places"
)

func TestParseClock(t *testing.T) {
	tests := map[string]*Clock{
		"09:00": {9, 0}, "9": {9, 0}, "9am": {9, 0}, "9pm": {21, 0}, "12am": {0, 0},
		"12pm": {12, 0}, "9:30pm": {21, 30}, "21h": {21, 0}, "9h30": {9, 30}, "9.30": {9, 30},
		"1730": {17, 30}, "noon": {12, 0}, "midnight": {0, 0}, "11:59p": {23, 59},
		"25": nil, "13pm": nil, "9:75": nil, "tokyo": nil, "2400": nil,
	}
	for in, want := range tests {
		got, ok := parseClock(in)
		if want == nil {
			if ok {
				t.Errorf("parseClock(%q) = %v, want no match", in, got)
			}
			continue
		}
		if !ok || got != *want {
			t.Errorf("parseClock(%q) = %v %v, want %v", in, got, ok, *want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in     string
		clock  *Clock
		places []string
	}{
		{"mst 09:00", &Clock{9, 0}, []string{"Denver"}},
		{"09:00 mst", &Clock{9, 0}, []string{"Denver"}},
		{"9 am pst", &Clock{9, 0}, []string{"Los Angeles"}},
		{"9am in salt lake city", &Clock{9, 0}, []string{"Salt Lake City"}},
		{"salt lake city tokyo", nil, []string{"Salt Lake City", "Tokyo"}},
		{"new york london", nil, []string{"New York City", "London"}},
		{"tokyo, valencia, spain", nil, []string{"Tokyo", "Valencia"}},
		{"15:30 utc+5:30", &Clock{15, 30}, []string{"UTC+5:30"}},
		{"in", nil, []string{"New Delhi"}}, // a lone filler still resolves (India)
		{"tomorrow 9am", &Clock{9, 0}, nil},
	}
	for _, tt := range tests {
		q, err := Parse(strings.Fields(tt.in))
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if (q.Clock == nil) != (tt.clock == nil) || (q.Clock != nil && *q.Clock != *tt.clock) {
			t.Errorf("Parse(%q).Clock = %v, want %v", tt.in, q.Clock, tt.clock)
		}
		var names []string
		for _, m := range q.Places {
			names = append(names, m.Place.Name)
		}
		if strings.Join(names, "|") != strings.Join(tt.places, "|") {
			t.Errorf("Parse(%q) places = %v, want %v", tt.in, names, tt.places)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"mts 09:00", "9am 10am tokyo", "tomorrow fri"} {
		if _, err := Parse(strings.Fields(in)); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

var (
	berlin = places.Place{Name: "Berlin", Sub: "Germany", TZ: "Europe/Berlin"}
	// Tue Oct 13 2026, 11:00 CEST
	now = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
)

func resolve(t *testing.T, in string) Result {
	t.Helper()
	q, err := Parse(strings.Fields(in))
	if err != nil {
		t.Fatalf("Parse(%q): %v", in, err)
	}
	r, err := q.Resolve(berlin, now, false)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", in, err)
	}
	return r
}

func TestResolveTimes(t *testing.T) {
	tests := []struct {
		in     string
		utc    string // expected focus, in UTC
		origin string
	}{
		{"mst 09:00", "2026-10-13 16:00", "Phoenix"},              // MST is UTC-7; Arizona has it in October
		{"9am pst", "2026-10-13 17:00", "PST"},                    // strict UTC-8, not PDT
		{"9am pt", "2026-10-13 16:00", "Los Angeles"},             // generic follows PDT
		{"9am pdt", "2026-10-13 16:00", "Los Angeles"},            //
		{"09:00", "2026-10-13 07:00", "Berlin"},                   // home
		{"tomorrow 15:30 tokyo", "2026-10-14 06:30", "Tokyo"},     //
		{"fri 14:00 utc+2", "2026-10-16 12:00", "UTC+2"},          // the coming Friday
		{"2026-10-25 12:00", "2026-10-25 11:00", "Berlin"},        // after DST ends: CET
		{"2026-10-24 12:00", "2026-10-24 10:00", "Berlin"},        // before: CEST
		{"9am ist", "2026-10-13 03:30", "Kolkata"},                //
		{"9am kathmandu", "2026-10-13 03:15", "Kathmandu"},        //
		{"11pm la", "2026-10-14 06:00", "Los Angeles"},            // crosses midnight in UTC
		{"9am cet", "2026-10-13 08:00", "CET"},                    // strict UTC+1 even in summer
		{"2026-01-15 9am mst", "2026-01-15 16:00", "Denver"},      // winter: Denver is on MST
		{"2026-01-15 9am pst", "2026-01-15 17:00", "Los Angeles"}, // winter: LA is on PST
	}
	for _, tt := range tests {
		r := resolve(t, tt.in)
		if got := r.Focus.UTC().Format("2006-01-02 15:04"); got != tt.utc {
			t.Errorf("%q: focus %s UTC, want %s", tt.in, got, tt.utc)
		}
		if r.Origin.Name != tt.origin {
			t.Errorf("%q: origin %q, want %q", tt.in, r.Origin.Name, tt.origin)
		}
	}
}

func TestResolveNotes(t *testing.T) {
	r := resolve(t, "9am pst")
	if len(r.Notes) == 0 || !strings.Contains(r.Notes[0], "6:00 pm") && !strings.Contains(r.Notes[0], "18:00") {
		t.Errorf("9am pst notes = %q, want the PDT reading (18:00 in Berlin)", r.Notes)
	}
	r = resolve(t, "mst 09:00")
	if len(r.Notes) == 0 || !strings.Contains(r.Notes[0], "Denver") || !strings.Contains(r.Notes[0], "17:00") {
		t.Errorf("mst notes = %q, want the Denver reading (17:00 in Berlin)", r.Notes)
	}
	r = resolve(t, "9am pt")
	for _, n := range r.Notes {
		if strings.Contains(n, "isn't in use") {
			t.Errorf("generic PT should not warn about seasons: %q", n)
		}
	}
	r = resolve(t, "2026-01-15 9am pst")
	if len(r.Notes) != 0 {
		t.Errorf("PST in January needs no note, got %q", r.Notes)
	}
}
