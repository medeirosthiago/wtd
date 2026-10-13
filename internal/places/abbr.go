package places

import (
	"strings"
	"time"
)

// Abbr is a time zone abbreviation such as "PST" or a generic name like "PT".
//
// Specific abbreviations mean a fixed offset: "9:00 PST" is UTC-8 even in
// summer, when Los Angeles is actually on PDT. Generic ones (Generic=true)
// follow the region's clock, daylight saving included.
type Abbr struct {
	Code    string
	Name    string
	Offset  int      // seconds east of UTC; unused when Generic
	Generic bool     // follows Zones[0], DST included
	Zones   []string // zones that use this abbreviation; Zones[0] is the region
	Alt     string   // other common meanings, shown as a hint
}

const hour = 3600

var abbrs = []Abbr{
	// Generic North American names, the way people write "9am ET".
	{Code: "PT", Name: "Pacific Time", Generic: true, Zones: zs("America/Los_Angeles"), Alt: "for Portugal use 'portugal'"},
	{Code: "MT", Name: "Mountain Time", Generic: true, Zones: zs("America/Denver"), Alt: "for Malta use 'malta'"},
	{Code: "CT", Name: "Central Time", Generic: true, Zones: zs("America/Chicago")},
	{Code: "ET", Name: "Eastern Time", Generic: true, Zones: zs("America/New_York"), Alt: "for Ethiopia use 'ethiopia'"},
	{Code: "AKT", Name: "Alaska Time", Generic: true, Zones: zs("America/Anchorage")},

	// Americas
	{Code: "HST", Name: "Hawaii Standard Time", Offset: -10 * hour, Zones: zs("Pacific/Honolulu")},
	{Code: "HDT", Name: "Hawaii-Aleutian Daylight Time", Offset: -9 * hour, Zones: zs("America/Adak")},
	{Code: "AKST", Name: "Alaska Standard Time", Offset: -9 * hour, Zones: zs("America/Anchorage")},
	{Code: "AKDT", Name: "Alaska Daylight Time", Offset: -8 * hour, Zones: zs("America/Anchorage")},
	{Code: "PST", Name: "Pacific Standard Time", Offset: -8 * hour, Zones: zs("America/Los_Angeles", "America/Vancouver", "America/Tijuana")},
	{Code: "PDT", Name: "Pacific Daylight Time", Offset: -7 * hour, Zones: zs("America/Los_Angeles", "America/Vancouver")},
	{Code: "MST", Name: "Mountain Standard Time", Offset: -7 * hour, Zones: zs("America/Denver", "America/Phoenix", "America/Edmonton")},
	{Code: "MDT", Name: "Mountain Daylight Time", Offset: -6 * hour, Zones: zs("America/Denver", "America/Edmonton")},
	{Code: "CST", Name: "Central Standard Time", Offset: -6 * hour, Zones: zs("America/Chicago", "America/Winnipeg"), Alt: "China Standard Time is UTC+8, use 'china' or 'shanghai'"},
	{Code: "CDT", Name: "Central Daylight Time", Offset: -5 * hour, Zones: zs("America/Chicago", "America/Winnipeg")},
	{Code: "EST", Name: "Eastern Standard Time", Offset: -5 * hour, Zones: zs("America/New_York", "America/Toronto")},
	{Code: "EDT", Name: "Eastern Daylight Time", Offset: -4 * hour, Zones: zs("America/New_York", "America/Toronto")},
	{Code: "AST", Name: "Atlantic Standard Time", Offset: -4 * hour, Zones: zs("America/Halifax", "America/Puerto_Rico"), Alt: "Arabia Standard Time is UTC+3, use 'riyadh'"},
	{Code: "ADT", Name: "Atlantic Daylight Time", Offset: -3 * hour, Zones: zs("America/Halifax")},
	{Code: "NST", Name: "Newfoundland Standard Time", Offset: -3*hour - 1800, Zones: zs("America/St_Johns")},
	{Code: "NDT", Name: "Newfoundland Daylight Time", Offset: -2*hour - 1800, Zones: zs("America/St_Johns")},
	{Code: "BRT", Name: "Brasília Time", Offset: -3 * hour, Zones: zs("America/Sao_Paulo")},
	{Code: "ART", Name: "Argentina Time", Offset: -3 * hour, Zones: zs("America/Argentina/Buenos_Aires")},
	{Code: "UYT", Name: "Uruguay Time", Offset: -3 * hour, Zones: zs("America/Montevideo")},
	{Code: "CLT", Name: "Chile Standard Time", Offset: -4 * hour, Zones: zs("America/Santiago")},
	{Code: "CLST", Name: "Chile Summer Time", Offset: -3 * hour, Zones: zs("America/Santiago")},
	{Code: "BOT", Name: "Bolivia Time", Offset: -4 * hour, Zones: zs("America/La_Paz")},
	{Code: "VET", Name: "Venezuela Time", Offset: -4 * hour, Zones: zs("America/Caracas")},
	{Code: "COT", Name: "Colombia Time", Offset: -5 * hour, Zones: zs("America/Bogota")},
	{Code: "PET", Name: "Peru Time", Offset: -5 * hour, Zones: zs("America/Lima")},

	// Europe & Africa
	{Code: "WET", Name: "Western European Time", Offset: 0, Zones: zs("Europe/Lisbon", "Atlantic/Canary")},
	{Code: "WEST", Name: "Western European Summer Time", Offset: 1 * hour, Zones: zs("Europe/Lisbon", "Atlantic/Canary")},
	{Code: "BST", Name: "British Summer Time", Offset: 1 * hour, Zones: zs("Europe/London"), Alt: "Bangladesh Standard Time is UTC+6, use 'dhaka'"},
	{Code: "IST", Name: "India Standard Time", Offset: 5*hour + 1800, Zones: zs("Asia/Kolkata"), Alt: "Israel uses IST/IDT (use 'israel'), Irish Standard Time is UTC+1 (use 'dublin')"},
	{Code: "CET", Name: "Central European Time", Offset: 1 * hour, Zones: zs("Europe/Berlin", "Europe/Paris")},
	{Code: "CEST", Name: "Central European Summer Time", Offset: 2 * hour, Zones: zs("Europe/Berlin", "Europe/Paris")},
	{Code: "EET", Name: "Eastern European Time", Offset: 2 * hour, Zones: zs("Europe/Athens", "Europe/Helsinki", "Europe/Kyiv")},
	{Code: "EEST", Name: "Eastern European Summer Time", Offset: 3 * hour, Zones: zs("Europe/Athens", "Europe/Helsinki", "Europe/Kyiv")},
	{Code: "MSK", Name: "Moscow Time", Offset: 3 * hour, Zones: zs("Europe/Moscow")},
	{Code: "TRT", Name: "Turkey Time", Offset: 3 * hour, Zones: zs("Europe/Istanbul")},
	{Code: "WAT", Name: "West Africa Time", Offset: 1 * hour, Zones: zs("Africa/Lagos")},
	{Code: "CAT", Name: "Central Africa Time", Offset: 2 * hour, Zones: zs("Africa/Maputo")},
	{Code: "SAST", Name: "South Africa Standard Time", Offset: 2 * hour, Zones: zs("Africa/Johannesburg")},
	{Code: "EAT", Name: "East Africa Time", Offset: 3 * hour, Zones: zs("Africa/Nairobi")},

	// Asia
	{Code: "IRST", Name: "Iran Standard Time", Offset: 3*hour + 1800, Zones: zs("Asia/Tehran")},
	{Code: "GST", Name: "Gulf Standard Time", Offset: 4 * hour, Zones: zs("Asia/Dubai")},
	{Code: "AFT", Name: "Afghanistan Time", Offset: 4*hour + 1800, Zones: zs("Asia/Kabul")},
	{Code: "PKT", Name: "Pakistan Standard Time", Offset: 5 * hour, Zones: zs("Asia/Karachi")},
	{Code: "NPT", Name: "Nepal Time", Offset: 5*hour + 2700, Zones: zs("Asia/Kathmandu")},
	{Code: "ICT", Name: "Indochina Time", Offset: 7 * hour, Zones: zs("Asia/Bangkok", "Asia/Ho_Chi_Minh")},
	{Code: "WIB", Name: "Western Indonesia Time", Offset: 7 * hour, Zones: zs("Asia/Jakarta")},
	{Code: "SGT", Name: "Singapore Time", Offset: 8 * hour, Zones: zs("Asia/Singapore")},
	{Code: "HKT", Name: "Hong Kong Time", Offset: 8 * hour, Zones: zs("Asia/Hong_Kong")},
	{Code: "PHT", Name: "Philippine Time", Offset: 8 * hour, Zones: zs("Asia/Manila")},
	{Code: "MYT", Name: "Malaysia Time", Offset: 8 * hour, Zones: zs("Asia/Kuala_Lumpur")},
	{Code: "WITA", Name: "Central Indonesia Time", Offset: 8 * hour, Zones: zs("Asia/Makassar")},
	{Code: "AWST", Name: "Australian Western Standard Time", Offset: 8 * hour, Zones: zs("Australia/Perth")},
	{Code: "JST", Name: "Japan Standard Time", Offset: 9 * hour, Zones: zs("Asia/Tokyo")},
	{Code: "KST", Name: "Korea Standard Time", Offset: 9 * hour, Zones: zs("Asia/Seoul")},
	{Code: "WIT", Name: "Eastern Indonesia Time", Offset: 9 * hour, Zones: zs("Asia/Jayapura")},
	{Code: "IDT", Name: "Israel Daylight Time", Offset: 3 * hour, Zones: zs("Asia/Jerusalem")},

	// Oceania
	{Code: "ACST", Name: "Australian Central Standard Time", Offset: 9*hour + 1800, Zones: zs("Australia/Adelaide", "Australia/Darwin")},
	{Code: "ACDT", Name: "Australian Central Daylight Time", Offset: 10*hour + 1800, Zones: zs("Australia/Adelaide")},
	{Code: "AEST", Name: "Australian Eastern Standard Time", Offset: 10 * hour, Zones: zs("Australia/Sydney", "Australia/Brisbane")},
	{Code: "AEDT", Name: "Australian Eastern Daylight Time", Offset: 11 * hour, Zones: zs("Australia/Sydney")},
	{Code: "NZST", Name: "New Zealand Standard Time", Offset: 12 * hour, Zones: zs("Pacific/Auckland")},
	{Code: "NZDT", Name: "New Zealand Daylight Time", Offset: 13 * hour, Zones: zs("Pacific/Auckland")},
}

var abbrAliases = map[string]string{
	"pacific":  "PT",
	"mountain": "MT",
	"central":  "CT",
	"eastern":  "ET",
	"alaska":   "AKT",
	"pst8pdt":  "PT",
	"est5edt":  "ET",
}

func zs(z ...string) []string { return z }

// LookupAbbr finds an abbreviation by code ("pst") or alias ("pacific").
func LookupAbbr(s string) (Abbr, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if code, ok := abbrAliases[s]; ok {
		s = strings.ToLower(code)
	}
	for _, a := range abbrs {
		if strings.ToLower(a.Code) == s {
			return a, true
		}
	}
	return Abbr{}, false
}

// ZoneAt returns the zone that actually shows this abbreviation's offset at t,
// e.g. MST in October is Phoenix (Denver is on MDT then). ok is false when no
// listed zone uses the offset at t, as with PST in summer.
func (a Abbr) ZoneAt(t time.Time) (zone string, ok bool) {
	if a.Generic {
		return a.Zones[0], true
	}
	for _, z := range a.Zones {
		loc, err := LoadLocation(z)
		if err != nil {
			continue
		}
		if _, off := t.In(loc).Zone(); off == a.Offset {
			return z, true
		}
	}
	return "", false
}

// knownAbbr names zones whose tzdata abbreviation is numeric, e.g.
// America/Sao_Paulo at UTC-3 is "BRT".
func knownAbbr(zone string, off int) (string, bool) {
	for _, a := range abbrs {
		if a.Generic || a.Offset != off {
			continue
		}
		for _, z := range a.Zones {
			if z == zone {
				return a.Code, true
			}
		}
	}
	return "", false
}
