// Package places turns user input ("tokyo", "pst", "utc+5:30", "brazil",
// "Europe/Berlin") into time zones and describes them for display.
package places

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // fallback when the system has no zoneinfo
)

// Place is a named time zone, as stored in the config file.
type Place struct {
	Name string `json:"name"`
	Sub  string `json:"sub,omitempty"` // e.g. "United States, Utah"
	TZ   string `json:"tz"`            // IANA name, or a fixed offset like "UTC+05:30"
}

var (
	locMu    sync.Mutex
	locCache = map[string]*time.Location{}
)

// Location loads the place's time zone.
func (p Place) Location() (*time.Location, error) {
	return LoadLocation(p.TZ)
}

// LoadLocation loads an IANA zone or a fixed "UTC±hh:mm" offset.
func LoadLocation(tz string) (*time.Location, error) {
	locMu.Lock()
	defer locMu.Unlock()
	if loc, ok := locCache[tz]; ok {
		return loc, nil
	}
	var loc *time.Location
	if off, ok := ParseOffset(tz); ok {
		loc = time.FixedZone(FormatOffset(off), off)
	} else {
		var err error
		if loc, err = time.LoadLocation(tz); err != nil {
			return nil, fmt.Errorf("unknown time zone %q", tz)
		}
	}
	locCache[tz] = loc
	return loc, nil
}

var offsetRe = regexp.MustCompile(`^(?:utc|gmt|z)?\s*(?:([+-])\s*(\d{1,2})(?::?(\d{2}))?)?$`)

// ParseOffset parses "UTC", "utc+2", "GMT-03:00", "+0530" or "-3" into seconds
// east of UTC.
func ParseOffset(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "zulu" {
		return 0, s == "zulu"
	}
	m := offsetRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	if m[1] == "" { // bare "utc", "gmt" or "z"
		return 0, true
	}
	h, _ := strconv.Atoi(m[2])
	mins := 0
	if m[3] != "" {
		mins, _ = strconv.Atoi(m[3])
	}
	if h > 14 || mins > 59 {
		return 0, false
	}
	off := h*3600 + mins*60
	if m[1] == "-" {
		off = -off
	}
	return off, true
}

// FormatOffset renders seconds east of UTC as "UTC", "UTC+2" or "UTC-9:30".
func FormatOffset(off int) string {
	if off == 0 {
		return "UTC"
	}
	return "UTC" + FormatDiff(off)
}

// FormatDiff renders an offset difference as "+6", "-5" or "+5:30".
func FormatDiff(d int) string {
	sign := "+"
	if d < 0 {
		sign, d = "-", -d
	}
	h, m := d/3600, d%3600/60
	if m == 0 {
		return fmt.Sprintf("%s%d", sign, h)
	}
	return fmt.Sprintf("%s%d:%02d", sign, h, m)
}

// OffsetTZ is how fixed offsets are stored in the config ("UTC-08:00").
func OffsetTZ(off int) string {
	if off == 0 {
		return "UTC"
	}
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, off/3600, off%3600/60)
}

// Tag is the short zone label shown next to a place, e.g. "CEST", "PDT" or
// "UTC-3" for zones whose tzdata abbreviation is numeric.
func Tag(t time.Time, loc *time.Location) string {
	name, off := t.In(loc).Zone()
	if isAlpha(name) {
		return name
	}
	if a, ok := knownAbbr(loc.String(), off); ok {
		return a
	}
	return FormatOffset(off)
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}
