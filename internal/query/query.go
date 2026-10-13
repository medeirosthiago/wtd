// Package query understands "mst 09:00", "9am pt", "tomorrow 15:30 tokyo" or
// just "tokyo london": an optional clock time and day, plus places.
package query

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medeirosthiago/wtd/internal/places"
)

// Query is the parsed input. Without a clock, the current time is used.
type Query struct {
	Clock  *Clock
	Day    Day
	Places []places.Match
}

type Clock struct{ Hour, Min int }

// Day is relative to today at the place the time is given in.
type Day struct {
	Offset  int           // "tomorrow" = 1
	Weekday *time.Weekday // "fri" = the coming Friday (today counts)
	Date    *time.Time    // "2026-10-20"
}

func (d Day) set() bool { return d.Offset != 0 || d.Weekday != nil || d.Date != nil }

// HasTime reports whether the query moves away from "now".
func (q Query) HasTime() bool { return q.Clock != nil || q.Day.set() }

var (
	clockRe    = regexp.MustCompile(`^(\d{1,2})(?:[:.h](\d{2}))?(am|pm|a|p)?$`)
	hourOnlyRe = regexp.MustCompile(`^(\d{1,2})h$`)
	militaryRe = regexp.MustCompile(`^([01]\d|2[0-3])([0-5]\d)$`)
	isoDateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	numRe      = regexp.MustCompile(`^\d{1,2}(?:[:.]\d{2})?$`)
)

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// Words that glue a query together ("9am in tokyo") and are never places
// on their own.
var fillers = map[string]bool{
	"at": true, "in": true, "on": true, "to": true, "vs": true, "and": true,
	"@": true, "->": true, "→": true, "=": true, "next": true, "this": true,
}

// Parse reads command-line words. Places may span several words ("salt lake
// city") and several places can follow each other ("tokyo london").
func Parse(args []string) (Query, error) {
	tokens := tokenize(args)
	var q Query
	var rest []string
	for _, tok := range tokens {
		low := strings.ToLower(tok)
		if c, ok := parseClock(low); ok {
			if q.Clock != nil {
				return q, fmt.Errorf("two times in one query: %q", tok)
			}
			q.Clock = &c
			continue
		}
		if d, ok := parseDay(low); ok {
			if q.Day.set() {
				return q, fmt.Errorf("two days in one query: %q", tok)
			}
			q.Day = d
			continue
		}
		rest = append(rest, tok)
	}
	matches, err := segment(rest)
	if err != nil {
		return q, err
	}
	q.Places = matches
	return q, nil
}

// tokenize splits words, detaches commas and glues "9 am" into "9am".
func tokenize(args []string) []string {
	var out []string
	for _, f := range strings.Fields(strings.Join(args, " ")) {
		for f != "" {
			i := strings.Index(f, ",")
			if i < 0 {
				out = append(out, f)
				break
			}
			if i > 0 {
				out = append(out, f[:i])
			}
			out = append(out, ",")
			f = f[i+1:]
		}
	}
	var merged []string
	for _, tok := range out {
		low := strings.ToLower(strings.ReplaceAll(tok, ".", ""))
		if (low == "am" || low == "pm") && len(merged) > 0 && numRe.MatchString(merged[len(merged)-1]) {
			merged[len(merged)-1] += low
			continue
		}
		merged = append(merged, tok)
	}
	return merged
}

func parseClock(s string) (Clock, bool) {
	s = strings.ReplaceAll(s, "a.m.", "am")
	s = strings.ReplaceAll(s, "p.m.", "pm")
	switch s {
	case "noon", "midday":
		return Clock{12, 0}, true
	case "midnight":
		return Clock{0, 0}, true
	}
	var h, m int
	var suffix string
	if mm := clockRe.FindStringSubmatch(s); mm != nil {
		h, _ = strconv.Atoi(mm[1])
		if mm[2] != "" {
			m, _ = strconv.Atoi(mm[2])
		}
		suffix = mm[3]
	} else if mm := hourOnlyRe.FindStringSubmatch(s); mm != nil {
		h, _ = strconv.Atoi(mm[1])
	} else if mm := militaryRe.FindStringSubmatch(s); mm != nil {
		h, _ = strconv.Atoi(mm[1])
		m, _ = strconv.Atoi(mm[2])
	} else {
		return Clock{}, false
	}
	if m > 59 {
		return Clock{}, false
	}
	switch suffix {
	case "am", "a":
		if h < 1 || h > 12 {
			return Clock{}, false
		}
		h %= 12
	case "pm", "p":
		if h < 1 || h > 12 {
			return Clock{}, false
		}
		h = h%12 + 12
	default:
		if h > 23 {
			return Clock{}, false
		}
	}
	return Clock{h, m}, true
}

func parseDay(s string) (Day, bool) {
	switch s {
	case "today", "now":
		return Day{}, s == "today"
	case "tomorrow", "tmrw", "tmr", "tom":
		return Day{Offset: 1}, true
	case "yesterday":
		return Day{Offset: -1}, true
	}
	if wd, ok := weekdays[s]; ok {
		return Day{Weekday: &wd}, true
	}
	if isoDateRe.MatchString(s) {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return Day{Date: &t}, true
		}
	}
	return Day{}, false
}

// segment resolves words into places, preferring the longest run of words
// that makes a place: "salt lake city tokyo" is two places, not four.
func segment(tokens []string) ([]places.Match, error) {
	var out []places.Match
	onlyFiller := len(tokens) == 1 && fillers[strings.ToLower(tokens[0])]
	for i := 0; i < len(tokens); {
		if tokens[i] == "," || (fillers[strings.ToLower(tokens[i])] && !onlyFiller) {
			i++
			continue
		}
		found := false
		for j := len(tokens); j > i; j-- {
			if tokens[j-1] == "," {
				continue
			}
			if m, ok := places.Lookup(strings.Join(tokens[i:j], " ")); ok {
				out = append(out, m)
				i, found = j, true
				break
			}
		}
		if !found {
			end := i + 1
			for end < len(tokens) && tokens[end] != "," {
				end++
			}
			_, err := places.Resolve(strings.Join(tokens[i:end], " "))
			if err == nil {
				err = errors.New("could not understand " + strings.Join(tokens[i:end], " "))
			}
			return nil, err
		}
	}
	return out, nil
}
