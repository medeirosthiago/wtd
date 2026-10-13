// Package view renders the worldtimebuddy-style table.
package view

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/medeirosthiago/wtd/internal/places"
)

// Row is one place in the table. Rows[0] is home.
type Row struct {
	Place places.Place
	Loc   *time.Location
	Temp  bool // shown for this query only, not saved
}

type Options struct {
	Now      time.Time
	Focus    time.Time // the highlighted instant; Now unless browsing or querying
	Hour12   bool
	Width    int // terminal width; the hour strip is windowed to fit
	Selected int // highlighted row in browse mode, -1 for none
	Dark     bool
	Compact  bool // drop the weekday label lines to save height
}

const (
	cellW    = 3
	maxNameW = 30
)

// Table renders the date tabs and one three-line block per row: a label line
// (weekday above midnight), the hour line and the am/pm line.
func Table(rows []Row, o Options) string {
	t := newTheme(o.Dark)
	home := rows[0]

	fh := o.Focus.In(home.Loc)
	dayStart := time.Date(fh.Year(), fh.Month(), fh.Day(), 0, 0, 0, 0, home.Loc)
	dayEnd := time.Date(fh.Year(), fh.Month(), fh.Day()+1, 0, 0, 0, 0, home.Loc)
	hours := int(dayEnd.Sub(dayStart) / time.Hour) // 23 or 25 on DST days
	focusCol := min(max(int(o.Focus.Sub(dayStart)/time.Hour), 0), hours-1)

	_, homeOff := o.Focus.In(home.Loc).Zone()

	// Left panel, measured first so the hour strip can use the rest.
	type left struct{ mark, name1, name2, time1, time2 string }
	lefts := make([]left, len(rows))
	markW, nameW, timeW := 1, 0, 0
	for i, r := range rows {
		at := o.Focus.In(r.Loc)
		var l left
		if i == 0 {
			l.mark = t.home.Render("⌂")
		} else {
			_, off := at.Zone()
			l.mark = t.mark.Render(diffLabel(off - homeOff))
		}
		name := r.Place.Name
		tag := places.Tag(o.Focus, r.Loc)
		if tag == name {
			tag = ""
		}
		l.name1 = name
		if tag != "" {
			l.name1 += " " + tag
		}
		l.name2 = r.Place.Sub
		l.time1 = clock(at, o.Hour12)
		l.time2 = at.Format("Mon, Jan 2")

		markW = max(markW, lipgloss.Width(l.mark))
		nameW = max(nameW, ansi.StringWidth(l.name1), ansi.StringWidth(l.name2))
		timeW = max(timeW, ansi.StringWidth(l.time1), ansi.StringWidth(l.time2))
		lefts[i] = l
	}
	nameW = min(nameW, maxNameW)
	leftW := 2 + markW + 2 + nameW + 2 + timeW + 3

	// Window the strip around the focus column when the terminal is narrow.
	cols := hours
	if o.Width > 0 {
		cols = min(hours, max((o.Width-leftW-1)/cellW, 6))
	}
	first := min(max(focusCol-cols/2, 0), hours-cols)

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", leftW))
	b.WriteString(dateTabs(t, fh, cols*cellW))
	b.WriteString("\n")

	for i, r := range rows {
		l := lefts[i]
		gutter := "  "
		if i == o.Selected {
			gutter = t.cursor.Render("▌ ")
		}
		nameStyle, subStyle := t.name, t.sub
		if r.Temp {
			nameStyle, subStyle = nameStyle.Inherit(t.temp), subStyle.Inherit(t.temp)
		}

		// Split "Berlin CEST" back into name and tag for styling.
		name1 := ansi.Truncate(l.name1, nameW, "…")
		var line1Name string
		if tag := places.Tag(o.Focus, r.Loc); tag != r.Place.Name && strings.HasSuffix(name1, " "+tag) {
			line1Name = nameStyle.Render(strings.TrimSuffix(name1, " "+tag)) + " " + t.tag.Render(tag)
		} else {
			line1Name = nameStyle.Render(name1)
		}
		name2 := subStyle.Render(ansi.Truncate(l.name2, nameW, "…"))

		at := o.Focus.In(r.Loc)
		clockText := t.clock.Render(clockDigits(at, o.Hour12))
		if o.Hour12 {
			clockText += t.ampm.Render(strings.ToLower(at.Format(" pm")))
		}

		labels, top, bottom := strip(t, r.Loc, dayStart, first, cols, focusCol, o.Hour12)

		if !o.Compact {
			b.WriteString(strings.Repeat(" ", leftW) + labels + "\n")
		}
		b.WriteString(gutter + padLeft(l.mark, markW) + "  " + padRight(line1Name, nameW) + "  " +
			padLeft(clockText, timeW) + "   " + top + "\n")
		b.WriteString(gutter + strings.Repeat(" ", markW) + "  " + padRight(name2, nameW) + "  " +
			padLeft(t.date.Render(l.time2), timeW) + "   " + bottom + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// strip renders one row's hour cells for columns [first, first+cols).
func strip(t theme, loc *time.Location, dayStart time.Time, first, cols, focusCol int, hour12 bool) (labels, top, bottom string) {
	var lb, tb, bb strings.Builder
	for c := first; c < first+cols; c++ {
		at := dayStart.Add(time.Duration(c) * time.Hour).In(loc)
		prev := at.Add(-time.Hour)
		newDay := at.Day() != prev.Day()

		style := t.hourStyle(at.Hour())
		var l1, l2 string
		switch {
		case newDay:
			style = t.midnight
			l1, l2 = strings.ToUpper(at.Format("Jan")), at.Format("2")
		default:
			l1 = hourLabel(at.Hour(), hour12) + fraction(at.Minute())
			if hour12 {
				l2 = strings.ToLower(at.Format("pm"))
			}
		}
		if c == focusCol {
			style = t.focus
		}

		label := strings.Repeat(" ", cellW)
		switch {
		case newDay:
			label = t.label.Render(strings.ToUpper(at.Format("Mon")))
		case c == focusCol:
			label = t.cursor.Render(center("▾", cellW))
		}
		lb.WriteString(label)
		tb.WriteString(style.Render(center(l1, cellW)))
		bb.WriteString(style.Render(center(l2, cellW)))
	}
	return lb.String(), tb.String(), bb.String()
}

// dateTabs renders "12  Oct 13  14  15  16  17  18" with weekends in red.
func dateTabs(t theme, day time.Time, width int) string {
	var parts []string
	used := 0
	for d := -1; d <= 5; d++ {
		at := day.AddDate(0, 0, d)
		label := at.Format("2")
		style := t.tab
		if d == 0 {
			label = at.Format("Mon, Jan 2")
			style = t.tabActive
		} else if wd := at.Weekday(); wd == time.Saturday || wd == time.Sunday {
			style = t.weekend
		}
		if used+len(label)+3 > width {
			break
		}
		used += len(label) + 3
		parts = append(parts, style.Render(label))
	}
	return strings.Join(parts, "   ")
}

func clock(at time.Time, hour12 bool) string {
	if hour12 {
		return clockDigits(at, true) + strings.ToLower(at.Format(" pm"))
	}
	return clockDigits(at, false)
}

func clockDigits(at time.Time, hour12 bool) string {
	if hour12 {
		return at.Format("3:04")
	}
	return at.Format("15:04")
}

func hourLabel(h int, hour12 bool) string {
	if !hour12 {
		return fmt.Sprint(h)
	}
	if h%12 == 0 {
		return "12"
	}
	return fmt.Sprint(h % 12)
}

// fraction marks zones off the hour grid: India is "9½", Nepal "9¾".
func fraction(min int) string {
	switch min {
	case 0:
		return ""
	case 15:
		return "¼"
	case 30:
		return "½"
	case 45:
		return "¾"
	}
	return fmt.Sprintf(":%02d", min)
}

func diffLabel(d int) string {
	if d == 0 {
		return "±0"
	}
	return places.FormatDiff(d)
}

func center(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return s
	}
	// Lean left so "11" and a following "OCT" keep a gap: "11 OCT".
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}

func padRight(s string, w int) string {
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

func padLeft(s string, w int) string {
	return strings.Repeat(" ", max(w-lipgloss.Width(s), 0)) + s
}

// Rows lays out home, then extra (query) places that aren't saved, then the
// other saved places.
func Rows(saved, extra []places.Place) ([]Row, error) {
	var rows []Row
	seen := map[places.Place]bool{}
	push := func(p places.Place, temp bool) error {
		if seen[p] {
			return nil
		}
		loc, err := p.Location()
		if err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		seen[p] = true
		rows = append(rows, Row{Place: p, Loc: loc, Temp: temp})
		return nil
	}
	isSaved := map[places.Place]bool{}
	for _, p := range saved {
		isSaved[p] = true
	}
	if err := push(saved[0], false); err != nil {
		return nil, err
	}
	for _, p := range extra {
		if !isSaved[p] {
			if err := push(p, true); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range saved[1:] {
		if err := push(p, false); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// Find returns the index of the row showing p, or 0 (home).
func Find(rows []Row, p places.Place) int {
	for i, r := range rows {
		if r.Place == p {
			return i
		}
	}
	return 0
}
