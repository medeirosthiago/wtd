package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/medeirosthiago/wtd/internal/places"
)

// Result is a query placed on the timeline.
type Result struct {
	Focus     time.Time
	Origin    places.Place // where the clock time was given; home when no place
	OriginLoc *time.Location
	Rows      []places.Place // places this query adds to the view, in order
	Notes     []string
}

// Resolve reads the clock time at the first place (or home) and works out
// which rows to show. A specific abbreviation is taken literally: "9:00 PST"
// is UTC-8 even in summer, with a note about what Pacific Time would give.
func (q Query) Resolve(home places.Place, now time.Time, hour12 bool) (Result, error) {
	homeLoc, err := home.Location()
	if err != nil {
		return Result{}, err
	}
	r := Result{Focus: now, Origin: home, OriginLoc: homeLoc}

	loc := homeLoc
	if len(q.Places) > 0 {
		m := q.Places[0]
		if a := m.Abbr; a != nil && !a.Generic {
			loc = time.FixedZone(a.Code, a.Offset)
		} else if loc, err = m.Place.Location(); err != nil {
			return Result{}, err
		}
	}
	r.Focus = q.at(now, loc)

	for i, m := range q.Places {
		p := m.Place
		if a := m.Abbr; a != nil && !a.Generic {
			zone, ok := a.ZoneAt(r.Focus)
			if ok {
				p = places.PlaceForZone(zone)
			} else {
				p = places.Place{Name: a.Code, Sub: a.Name, TZ: places.OffsetTZ(a.Offset)}
			}
			if zone != a.Zones[0] {
				r.Notes = append(r.Notes, q.offSeason(*a, p, r.Focus, homeLoc, hour12))
			}
		}
		if m.Note != "" {
			r.Notes = append(r.Notes, m.Note)
		}
		pl, err := p.Location()
		if err != nil {
			return Result{}, err
		}
		if i == 0 {
			r.Origin, r.OriginLoc = p, pl
		}
		r.Rows = append(r.Rows, p)
	}
	return r, nil
}

// at applies the day and clock to now, as seen in loc.
func (q Query) at(now time.Time, loc *time.Location) time.Time {
	base := now.In(loc)
	y, m, d := base.Date()
	switch {
	case q.Day.Date != nil:
		y, m, d = q.Day.Date.Date()
	case q.Day.Weekday != nil:
		d += (int(*q.Day.Weekday) - int(base.Weekday()) + 7) % 7
	default:
		d += q.Day.Offset
	}
	if q.Clock == nil {
		return time.Date(y, m, d, base.Hour(), base.Minute(), base.Second(), 0, loc)
	}
	return time.Date(y, m, d, q.Clock.Hour, q.Clock.Min, 0, 0, loc)
}

// offSeason explains a standard/daylight abbreviation read somewhere other
// than its usual region: "PST" in October (Los Angeles is on PDT), or "MST"
// in summer (Phoenix has it, Denver is on MDT). People often mean the region.
func (q Query) offSeason(a places.Abbr, used places.Place, focus time.Time, homeLoc *time.Location, hour12 bool) string {
	region := places.PlaceForZone(a.Zones[0])
	regionLoc, err := region.Location()
	if err != nil {
		return ""
	}
	_, regionOff := focus.In(regionLoc).Zone()
	regionTag := places.Tag(focus, regionLoc)
	code := fmt.Sprintf("%s (%s)", a.Code, places.FormatOffset(a.Offset))
	var msg string
	if used.Name == a.Code {
		msg = fmt.Sprintf("%s isn't in use %s: %s is on %s (%s).", code, when(q, focus.In(regionLoc)), region.Name, regionTag, places.FormatOffset(regionOff))
	} else {
		msg = fmt.Sprintf("%s is %s's time %s; %s is on %s (%s).", code, used.Name, when(q, focus.In(regionLoc)), region.Name, regionTag, places.FormatOffset(regionOff))
	}
	if q.Clock != nil {
		// Same wall clock and date as written, read in the region instead.
		y, m, d := focus.In(time.FixedZone(a.Code, a.Offset)).Date()
		alt := time.Date(y, m, d, q.Clock.Hour, q.Clock.Min, 0, 0, regionLoc)
		_, altOff := alt.Zone()
		if _, homeOff := alt.In(homeLoc).Zone(); homeOff == altOff {
			msg += fmt.Sprintf(" If they meant %s time, that's %s %s, same as yours.", region.Name, Clock12(alt, hour12), regionTag)
		} else {
			msg += fmt.Sprintf(" If they meant %s time, %s %s is %s %s for you.",
				region.Name, Clock12(alt, hour12), regionTag, Clock12(alt.In(homeLoc), hour12), places.Tag(alt, homeLoc))
		}
	}
	return msg
}

func when(q Query, t time.Time) string {
	if q.HasTime() {
		return "on " + t.Format("Mon, Jan 2")
	}
	return "right now"
}

// Clock12 formats a time of day in the user's 12h or 24h style.
func Clock12(t time.Time, hour12 bool) string {
	if hour12 {
		return strings.ToLower(t.Format("3:04 pm"))
	}
	return t.Format("15:04")
}
