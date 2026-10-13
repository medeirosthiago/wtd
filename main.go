// Command wtd is a terminal worldtimebuddy.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/medeirosthiago/wtd/internal/config"
	"github.com/medeirosthiago/wtd/internal/places"
	"github.com/medeirosthiago/wtd/internal/query"
	"github.com/medeirosthiago/wtd/internal/tui"
	"github.com/medeirosthiago/wtd/internal/view"
)

const usage = `wtd - world time, buddy

  wtd                         your places, now
  wtd <time> [place]          what that time is for you and everyone else
                                wtd mst 09:00    wtd 9am pt    wtd 15:30 tokyo
                                wtd tomorrow 9am new york      wtd fri 14:00 utc+2
  wtd <place>...              your places plus these, for a quick look
                                wtd tokyo        wtd salt lake city, sydney
  wtd browse                  interactive: move through hours, add, query (also -i)

  wtd add <place>...          save places (comma or space separated)
  wtd rm <place|#>...         remove saved places
  wtd home <place|#>          set home; the old home stays in the list
  wtd ls                      list saved places
  wtd search <text>           show what a place name could mean
  wtd clock 12|24             am/pm or 24-hour clock
  wtd theme auto|light|dark   colours

Places can be cities ("rio", "valencia, spain", "portland me"), countries or
ISO codes ("brazil", "DE"), abbreviations ("pst", "cest", "ist", "pt"), UTC
offsets ("utc+5:30", "gmt-3") or IANA names ("America/Denver").

Flags: --12h, --24h, --light, --dark, -i/--browse, -h/--help
Config: %s
`

type flags struct {
	hour12, hour24, light, dark, browse, help bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "wtd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var f flags
	args = parseFlags(args, &f)
	if f.help {
		fmt.Printf(usage, config.Path())
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if f.hour12 {
		cfg.Clock = "12h"
	}
	if f.hour24 {
		cfg.Clock = "24h"
	}
	if f.light {
		cfg.Theme = "light"
	}
	if f.dark {
		cfg.Theme = "dark"
	}
	if f.browse {
		return browse(cfg, args)
	}
	if len(args) == 0 {
		return show(cfg, query.Query{})
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "add":
		return add(cfg, rest)
	case "rm", "remove", "del", "delete":
		return remove(cfg, rest)
	case "home":
		return setHome(cfg, rest)
	case "ls", "list":
		return list(cfg)
	case "search", "find":
		return search(cfg, rest)
	case "browse", "tui":
		return browse(cfg, rest)
	case "clock":
		return setClock(cfg, rest)
	case "theme":
		return setTheme(cfg, rest)
	case "help":
		fmt.Printf(usage, config.Path())
		return nil
	}
	q, err := query.Parse(args)
	if err != nil {
		return err
	}
	return show(cfg, q)
}

// parseFlags pulls known flags out; "-3" stays an argument (UTC offset).
func parseFlags(args []string, f *flags) []string {
	var rest []string
	for _, a := range args {
		switch a {
		case "--12h", "--12":
			f.hour12 = true
		case "--24h", "--24":
			f.hour24 = true
		case "--light":
			f.light = true
		case "--dark":
			f.dark = true
		case "-i", "--browse", "--interactive":
			f.browse = true
		case "-h", "--help":
			f.help = true
		default:
			rest = append(rest, a)
		}
	}
	return rest
}

func show(cfg *config.Config, q query.Query) error {
	now := time.Now()
	res, err := q.Resolve(cfg.Places[0], now, cfg.Hour12())
	if err != nil {
		return err
	}
	rows, err := view.Rows(cfg.Places, res.Rows)
	if err != nil {
		return err
	}
	origin := view.Find(rows, res.Origin)
	o := view.Options{
		Now:      now,
		Focus:    res.Focus,
		Hour12:   cfg.Hour12(),
		Width:    termWidth(),
		Selected: -1,
		Dark:     isDark(cfg),
	}
	if q.HasTime() {
		lipgloss.Println(view.Answer(rows[origin], rows[0], o) + "\n")
	}
	lipgloss.Println(view.Table(rows, o))
	if notes := view.Notes(res.Notes, o.Width, o.Dark); notes != "" {
		lipgloss.Println("\n" + notes)
	}
	return nil
}

func add(cfg *config.Config, args []string) error {
	q, err := placesOnly(args, "add")
	if err != nil {
		return err
	}
	for _, m := range q.Places {
		p := m.Place
		if !cfg.Add(p) {
			fmt.Printf("%s is already saved\n", p.Name)
			continue
		}
		fmt.Printf("Added %s\n", describe(p))
		if loc, _ := p.Location(); m.Abbr != nil && !m.Abbr.Generic && places.ObservesDST(loc, time.Now().Year()) {
			off := places.FormatOffset(m.Abbr.Offset)
			fmt.Printf("  %s's clock follows daylight saving; for %s as a fixed %s, add '%s'\n",
				p.Name, m.Abbr.Code, off, strings.ToLower(off))
		}
		if m.Note != "" {
			fmt.Printf("  note: %s\n", m.Note)
		}
	}
	return cfg.Save()
}

func remove(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wtd rm <place|#>...")
	}
	// Resolve all positions first so "wtd rm 2 3" means the current 2 and 3.
	var targets []places.Place
	for _, a := range splitArgs(args) {
		i := cfg.Index(a)
		if i < 0 {
			if m, err := places.Resolve(a); err == nil {
				i = cfg.Has(m.Place)
			}
		}
		if i < 0 {
			return fmt.Errorf("%q isn't in your list (see wtd ls)", a)
		}
		targets = append(targets, cfg.Places[i])
	}
	for _, p := range targets {
		wasHome := cfg.Places[0] == p
		if err := cfg.Remove(cfg.Has(p)); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", p.Name)
		if wasHome {
			fmt.Printf("Home is now %s\n", cfg.Places[0].Name)
		}
	}
	return cfg.Save()
}

func setHome(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Printf("Home is %s\n", describe(cfg.Places[0]))
		return nil
	}
	var p places.Place
	if i := cfg.Index(strings.Join(args, " ")); i >= 0 {
		p = cfg.Places[i]
	} else {
		q, err := placesOnly(args, "home")
		if err != nil {
			return err
		}
		if len(q.Places) != 1 {
			return errors.New("home takes one place")
		}
		p = q.Places[0].Place
	}
	cfg.SetHome(p)
	fmt.Printf("Home is now %s\n", describe(p))
	return cfg.Save()
}

func list(cfg *config.Config) error {
	now := time.Now()
	for i, p := range cfg.Places {
		loc, err := p.Location()
		if err != nil {
			return err
		}
		mark := " "
		if i == 0 {
			mark = "⌂"
		}
		fmt.Printf("%s %2d  %-22s %-28s %-24s %s\n", mark, i+1, p.Name, p.Sub, p.TZ,
			query.Clock12(now.In(loc), cfg.Hour12())+" "+places.Tag(now, loc))
	}
	return nil
}

func search(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: wtd search <text>")
	}
	q := strings.Join(args, " ")
	ms := places.Search(q, 12)
	if len(ms) == 0 {
		_, err := places.Resolve(q)
		return err
	}
	now := time.Now()
	for i, m := range ms {
		loc, err := m.Place.Location()
		if err != nil {
			continue
		}
		_, off := now.In(loc).Zone()
		pick := " "
		if i == 0 {
			pick = "›"
		}
		fmt.Printf("%s %-26s %-32s %-28s %-9s %-8s %s\n", pick, trunc(m.Place.Name, 26), trunc(m.Place.Sub, 32),
			m.Place.TZ, places.Tag(now, loc), places.FormatOffset(off), query.Clock12(now.In(loc), cfg.Hour12()))
	}
	if ms[0].Note != "" {
		fmt.Printf("\n  note: %s\n", ms[0].Note)
	}
	return nil
}

func setClock(cfg *config.Config, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: wtd clock 12|24")
	}
	switch strings.TrimSuffix(args[0], "h") {
	case "12":
		cfg.Clock = "12h"
	case "24":
		cfg.Clock = "24h"
	default:
		return errors.New("usage: wtd clock 12|24")
	}
	fmt.Printf("Clock: %s\n", cfg.Clock)
	return cfg.Save()
}

func setTheme(cfg *config.Config, args []string) error {
	if len(args) != 1 || (args[0] != "auto" && args[0] != "light" && args[0] != "dark") {
		return errors.New("usage: wtd theme auto|light|dark")
	}
	cfg.Theme = args[0]
	fmt.Printf("Theme: %s\n", cfg.Theme)
	return cfg.Save()
}

func browse(cfg *config.Config, args []string) error {
	var q query.Query
	if len(args) > 0 {
		var err error
		if q, err = query.Parse(args); err != nil {
			return err
		}
	}
	return tui.Run(cfg, q, isDark(cfg))
}

// placesOnly parses places for commands where a time makes no sense.
func placesOnly(args []string, cmd string) (query.Query, error) {
	if len(args) == 0 {
		return query.Query{}, fmt.Errorf("usage: wtd %s <place>...", cmd)
	}
	q, err := query.Parse(args)
	if err != nil {
		return q, err
	}
	if q.HasTime() {
		return q, fmt.Errorf("%s takes places, not times", cmd)
	}
	if len(q.Places) == 0 {
		return q, fmt.Errorf("usage: wtd %s <place>...", cmd)
	}
	return q, nil
}

// splitArgs splits on commas but keeps multi-word names together.
func splitArgs(args []string) []string {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, ",") {
		var out []string
		for _, s := range strings.Split(joined, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	// "wtd rm 2 3" removes two; "wtd rm salt lake city" removes one.
	allNums := true
	for _, a := range args {
		if _, err := strconv.Atoi(a); err != nil {
			allNums = false
		}
	}
	if allNums {
		return args
	}
	return []string{joined}
}

func describe(p places.Place) string {
	loc, err := p.Location()
	if err != nil {
		return p.Name
	}
	now := time.Now()
	_, off := now.In(loc).Zone()
	s := p.Name
	if p.Sub != "" {
		s += " (" + p.Sub + ")"
	}
	return fmt.Sprintf("%s · %s, %s", s, places.Tag(now, loc), places.FormatOffset(off))
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func termWidth() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w > 0 {
		return w
	}
	return 0 // not a terminal: show all 24 hours
}

func isDark(cfg *config.Config) bool {
	switch cfg.Theme {
	case "light":
		return false
	case "dark":
		return true
	}
	if !term.IsTerminal(os.Stdout.Fd()) || !term.IsTerminal(os.Stdin.Fd()) {
		return true
	}
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}
