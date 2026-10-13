package view

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/medeirosthiago/wtd/internal/places"
)

func testRows(t *testing.T, names ...string) []Row {
	t.Helper()
	var ps []places.Place
	for _, n := range names {
		m, err := places.Resolve(n)
		if err != nil {
			t.Fatal(err)
		}
		ps = append(ps, m.Place)
	}
	rows, err := Rows(ps, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestTableFitsWidth(t *testing.T) {
	rows := testRows(t, "berlin", "rio", "salt lake city", "kolkata", "taipei")
	now := time.Date(2026, 10, 13, 9, 11, 0, 0, time.UTC)
	for _, width := range []int{70, 80, 100, 120, 160} {
		out := Table(rows, Options{Now: now, Focus: now, Hour12: true, Width: width, Selected: -1})
		for i, line := range strings.Split(out, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line %d is %d wide: %q", width, i, w, ansi.Strip(line))
			}
		}
	}
}

func TestTableContent(t *testing.T) {
	rows := testRows(t, "berlin", "rio", "taipei")
	now := time.Date(2026, 10, 13, 9, 11, 0, 0, time.UTC) // 11:11 CEST, like the screenshot
	out := ansi.Strip(Table(rows, Options{Now: now, Focus: now, Hour12: true, Selected: -1}))
	lines := strings.Split(out, "\n")
	// tabs, then 3 lines per row
	if len(lines) != 1+3*len(rows) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), 1+3*len(rows), out)
	}
	for _, want := range []string{"Berlin CEST", "11:11 am", "Rio de Janeiro BRT", "6:11 am", "-5", "+6", "Taipei CST", "5:11 pm", "WED"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	// Berlin's strip starts at its midnight: the OCT/13 tile.
	if !strings.Contains(lines[2], "OCT 1 ") || !strings.Contains(lines[3], "13 am") {
		t.Errorf("Berlin strip should start with the OCT/13 tile:\n%s\n%s", lines[2], lines[3])
	}
}

func TestTableDSTDay(t *testing.T) {
	rows := testRows(t, "berlin")
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) // Europe falls back: 25 hours
	out := ansi.Strip(Table(rows, Options{Now: at, Focus: at, Hour12: false, Selected: -1}))
	hourLine := strings.Split(out, "\n")[2]
	if !strings.Contains(hourLine, " 2  2  3") {
		t.Errorf("fall-back day should repeat 2 o'clock: %q", hourLine)
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for d, want := range map[time.Duration]string{
		0:                            "now",
		6*time.Hour + 44*time.Minute: "in 6h 44m",
		-3 * time.Hour:               "3h ago",
		25 * time.Minute:             "in 25m",
		72 * time.Hour:               "in 3 days",
	} {
		if got := Relative(now, now.Add(d)); got != want {
			t.Errorf("Relative(%v) = %q, want %q", d, got, want)
		}
	}
}
