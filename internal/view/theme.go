package view

import (
	"charm.land/lipgloss/v2"
)

type theme struct {
	// Hour cells, by time of day at that place.
	night, twilight, day, midnight, focus lipgloss.Style

	name, sub, tag, clock, ampm, date, mark, home               lipgloss.Style
	label, weekend, tab, tabActive, faint, accent, answer, note lipgloss.Style
	cursor, temp                                                lipgloss.Style
}

func newTheme(dark bool) theme {
	c := lipgloss.Color
	ld := lipgloss.LightDark(dark)
	cell := func(bg, fg string) lipgloss.Style {
		return lipgloss.NewStyle().Background(c(bg)).Foreground(c(fg))
	}
	t := theme{
		focus:     lipgloss.NewStyle().Bold(true).Background(ld(c("#1F2533"), c("#F25D94"))).Foreground(c("#FFFFFF")),
		name:      lipgloss.NewStyle().Bold(true).Foreground(ld(c("#1F2533"), c("#ECEFF4"))),
		sub:       lipgloss.NewStyle().Foreground(ld(c("#6B7280"), c("#8B93A7"))),
		tag:       lipgloss.NewStyle().Foreground(ld(c("#7A8090"), c("#9AA3B8"))).Background(ld(c("#ECEEF2"), c("#2A3040"))),
		clock:     lipgloss.NewStyle().Bold(true).Foreground(ld(c("#1F2533"), c("#ECEFF4"))),
		ampm:      lipgloss.NewStyle().Foreground(ld(c("#4B5263"), c("#B8C0D4"))),
		date:      lipgloss.NewStyle().Foreground(ld(c("#4B5263"), c("#9AA3B8"))),
		mark:      lipgloss.NewStyle().Bold(true).Foreground(ld(c("#7D8C5A"), c("#A3B57A"))),
		home:      lipgloss.NewStyle().Foreground(ld(c("#1F2533"), c("#ECEFF4"))),
		label:     lipgloss.NewStyle().Foreground(ld(c("#7A8090"), c("#8B93A7"))),
		weekend:   lipgloss.NewStyle().Foreground(ld(c("#D9534F"), c("#F2777A"))),
		tab:       lipgloss.NewStyle().Foreground(ld(c("#4B5263"), c("#9AA3B8"))),
		tabActive: lipgloss.NewStyle().Bold(true).Underline(true).Foreground(ld(c("#1F2533"), c("#ECEFF4"))),
		faint:     lipgloss.NewStyle().Foreground(ld(c("#8A90A0"), c("#6B7389"))),
		accent:    lipgloss.NewStyle().Bold(true).Foreground(ld(c("#4A6FB5"), c("#8FB3F0"))),
		answer:    lipgloss.NewStyle().Bold(true).Foreground(ld(c("#1F2533"), c("#ECEFF4"))),
		note:      lipgloss.NewStyle().Foreground(ld(c("#9A6B10"), c("#E5C07B"))),
		cursor:    lipgloss.NewStyle().Bold(true).Foreground(ld(c("#E0457B"), c("#F25D94"))),
		temp:      lipgloss.NewStyle().Italic(true),
	}
	if dark {
		t.night = cell("#243052", "#93A3CB")
		t.midnight = cell("#3A4D85", "#FFFFFF").Bold(true)
		t.twilight = cell("#5A73B0", "#F0F4FF")
		t.day = cell("#E8E1B0", "#28324F")
	} else {
		// worldtimebuddy's palette
		t.night = cell("#93A7D8", "#FFFFFF")
		t.midnight = cell("#7489C2", "#FFFFFF").Bold(true)
		t.twilight = cell("#DDE8F7", "#5774B0")
		t.day = cell("#FFFCE3", "#5774B0")
	}
	return t
}

// hourStyle picks the cell colour like worldtimebuddy: night until 6, a
// twilight band 6-8 and 18-22, daylight in between.
func (t theme) hourStyle(h int) lipgloss.Style {
	switch {
	case h < 6 || h >= 22:
		return t.night
	case h < 8 || h >= 18:
		return t.twilight
	default:
		return t.day
	}
}
