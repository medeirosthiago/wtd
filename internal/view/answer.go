package view

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/medeirosthiago/wtd/internal/places"
)

// Answer is the one-line reply to a query:
//
//	9:00 am MST in Phoenix  →  6:00 pm CEST in Berlin ⌂   Tue, Oct 13 · in 6h 44m
func Answer(origin Row, home Row, o Options) string {
	t := newTheme(o.Dark)
	at := o.Focus.In(origin.Loc)
	hat := o.Focus.In(home.Loc)
	sameDay := at.Format("2006-01-02") == hat.Format("2006-01-02")

	side := func(r Row, at time.Time, withDate bool) string {
		tag := places.Tag(o.Focus, r.Loc)
		var s string
		if _, fixed := places.ParseOffset(r.Place.TZ); fixed {
			// "9:00 am PST (UTC-8)" or just "2:00 pm UTC+5:30"
			s = t.answer.Render(clock(at, o.Hour12) + " " + r.Place.Name)
			if r.Place.Name != tag {
				s += t.sub.Render(" (" + tag + ")")
			}
		} else {
			s = t.answer.Render(clock(at, o.Hour12)+" "+tag) + t.sub.Render(" in ") + t.name.Render(r.Place.Name)
		}
		if withDate {
			s += t.date.Render(" " + at.Format("Mon, Jan 2"))
		}
		return s
	}

	var b strings.Builder
	b.WriteString("  ")
	if origin.Place == home.Place {
		b.WriteString(side(home, hat, false) + " " + t.home.Render("⌂"))
	} else {
		b.WriteString(side(origin, at, !sameDay))
		b.WriteString(t.accent.Render("  →  "))
		b.WriteString(side(home, hat, !sameDay) + " " + t.home.Render("⌂"))
	}
	b.WriteString("   ")
	if sameDay {
		b.WriteString(t.date.Render(hat.Format("Mon, Jan 2")) + t.faint.Render(" · "))
	}
	b.WriteString(t.faint.Render(Relative(o.Now, o.Focus)))
	return b.String()
}

// Notes renders hints below the table, wrapped to width (0 = no wrapping).
func Notes(notes []string, width int, dark bool) string {
	t := newTheme(dark)
	var lines []string
	for _, n := range notes {
		if n == "" {
			continue
		}
		if width > 10 {
			n = lipgloss.Wrap(n, width-6, " ")
		}
		n = strings.ReplaceAll(n, "\n", "\n    ")
		lines = append(lines, t.note.Render("  ! "+n))
	}
	return strings.Join(lines, "\n")
}

// Relative says "in 6h 44m", "3h 5m ago" or "now".
func Relative(now, at time.Time) string {
	d := at.Sub(now).Round(time.Minute)
	ago := d < 0
	if ago {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		return "now"
	case d >= 48*time.Hour:
		s = fmt.Sprintf("%d days", int(d.Hours()/24+0.5))
	case d >= time.Hour:
		s = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		s = strings.TrimSuffix(s, " 0m")
	default:
		s = fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if ago {
		return s + " ago"
	}
	return "in " + s
}
