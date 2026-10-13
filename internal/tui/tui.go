// Package tui is the interactive browse mode: move through the hours, add
// and remove places, and ask "mst 09:00" style questions.
package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/medeirosthiago/wtd/internal/config"
	"github.com/medeirosthiago/wtd/internal/places"
	"github.com/medeirosthiago/wtd/internal/query"
	"github.com/medeirosthiago/wtd/internal/view"
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeAsk
)

type tickMsg time.Time

type model struct {
	cfg           *config.Config
	dark          bool
	width, height int

	now, focus time.Time
	follow     bool // focus tracks the clock

	sel    int
	temp   []places.Place // places from the last question, not saved
	origin *places.Place  // where the last question's time was given
	notes  []string

	mode    mode
	input   textinput.Model
	matches []places.Match // suggestions while adding
	pick    int
	preview string // live answer while asking

	help      bool
	status    string
	statusErr bool

	st styles
}

// Run starts browse mode, optionally with a question already applied
// ("wtd browse mst 09:00").
func Run(cfg *config.Config, q query.Query, dark bool) error {
	now := time.Now()
	st := newStyles(dark)
	in := textinput.New()
	inStyles := textinput.DefaultStyles(dark)
	inStyles.Focused.Prompt = st.key
	inStyles.Focused.Placeholder = st.faint
	in.SetStyles(inStyles)
	m := model{cfg: cfg, dark: dark, now: now, focus: now, follow: true, input: in, st: st}
	if len(q.Places) > 0 || q.HasTime() {
		if err := m.ask(q); err != nil {
			return err
		}
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m model) Init() tea.Cmd { return tick(m.now) }

// tick wakes up on the next minute so the clocks stay current.
func tick(now time.Time) tea.Cmd {
	return tea.Tick(time.Until(now.Truncate(time.Minute).Add(time.Minute)), func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) rows() []view.Row {
	rows, err := view.Rows(m.cfg.Places, m.temp)
	if err != nil {
		// Saved places were validated when added; fall back to home only.
		rows, _ = view.Rows(m.cfg.Places[:1], nil)
	}
	return rows
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(msg.Width-12, 20))
		return m, nil
	case tickMsg:
		m.now = time.Time(msg)
		if m.follow {
			m.focus = m.now
		}
		return m, tick(m.now)
	case tea.KeyPressMsg:
		m.status, m.statusErr = "", false
		if m.mode != modeNormal {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	}
	if m.mode != modeNormal {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.rows()
	m.sel = min(max(m.sel, 0), len(rows)-1)
	cur := rows[m.sel]

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.origin != nil || len(m.temp) > 0 || !m.follow {
			m.clearQuestion()
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.help = !m.help

	case "left", "h":
		m.move(-time.Hour)
	case "right", "l":
		m.move(time.Hour)
	case "shift+left", "<", ",":
		m.move(-15 * time.Minute)
	case "shift+right", ">", ".":
		m.move(15 * time.Minute)
	case "[":
		m.moveDays(-1)
	case "]":
		m.moveDays(1)
	case "n", "0", "home":
		m.follow, m.focus = true, m.now

	case "up", "k":
		m.sel = max(m.sel-1, 0)
	case "down", "j":
		m.sel = min(m.sel+1, len(rows)-1)
	case "shift+up", "K":
		m.reorder(cur, -1)
	case "shift+down", "J":
		m.reorder(cur, 1)

	case "a", "+":
		return m.startInput(modeAdd)
	case "/", ":":
		return m.startInput(modeAsk)

	case "H":
		if m.sel == 0 {
			m.setStatus(cur.Place.Name+" is already home", false)
			break
		}
		m.dropTemp(cur.Place)
		m.cfg.SetHome(cur.Place)
		m.save("Home is now " + cur.Place.Name)
		m.sel = 0
	case "p":
		if !cur.Temp {
			m.setStatus(cur.Place.Name+" is already saved", false)
			break
		}
		m.dropTemp(cur.Place)
		m.cfg.Add(cur.Place)
		m.save("Saved " + cur.Place.Name)
		m.sel = view.Find(m.rows(), cur.Place)
	case "d", "x", "delete", "backspace":
		if cur.Temp {
			m.dropTemp(cur.Place)
			break
		}
		if err := m.cfg.Remove(m.cfg.Has(cur.Place)); err != nil {
			m.setStatus(err.Error(), true)
			break
		}
		m.save("Removed " + cur.Place.Name)
	case "t":
		if m.cfg.Hour12() {
			m.cfg.Clock = "24h"
		} else {
			m.cfg.Clock = "12h"
		}
		m.save("Clock: " + m.cfg.Clock)
	}
	m.sel = min(max(m.sel, 0), len(m.rows())-1)
	return m, nil
}

func (m *model) move(d time.Duration) {
	m.follow = false
	m.focus = m.focus.Add(d)
}

func (m *model) moveDays(n int) {
	loc := m.rows()[0].Loc
	m.follow = false
	m.focus = m.focus.In(loc).AddDate(0, 0, n)
}

func (m *model) reorder(cur view.Row, delta int) {
	i := m.cfg.Has(cur.Place)
	if cur.Temp || i <= 0 {
		m.setStatus("only saved places below home can be moved; use H to change home", true)
		return
	}
	if j := i + delta; j < 1 || j >= len(m.cfg.Places) {
		return
	}
	m.cfg.Move(i, delta)
	m.save("")
	m.sel = view.Find(m.rows(), cur.Place)
}

func (m *model) dropTemp(p places.Place) {
	m.temp = slices.DeleteFunc(m.temp, func(t places.Place) bool { return t == p })
	if m.origin != nil && *m.origin == p {
		m.origin = nil
	}
}

func (m *model) clearQuestion() {
	m.temp, m.origin, m.notes = nil, nil, nil
	m.follow, m.focus = true, m.now
}

func (m *model) save(msg string) {
	if err := m.cfg.Save(); err != nil {
		m.setStatus("couldn't save: "+err.Error(), true)
		return
	}
	if msg != "" {
		m.setStatus(msg, false)
	}
}

func (m *model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

func (m model) startInput(md mode) (tea.Model, tea.Cmd) {
	m.mode = md
	m.input.Reset()
	m.matches, m.pick, m.preview = nil, 0, ""
	if md == modeAdd {
		m.input.Prompt = "add › "
		m.input.Placeholder = "city, country, abbreviation, utc+3, Europe/Paris…"
	} else {
		m.input.Prompt = "ask › "
		m.input.Placeholder = "mst 09:00 · 9am tokyo · tomorrow 15:30 new york · fri 14:00 utc+2"
	}
	return m, m.input.Focus()
}

func (m model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "up", "shift+tab":
		if m.mode == modeAdd && len(m.matches) > 0 {
			m.pick = (m.pick - 1 + len(m.matches)) % len(m.matches)
		}
		return m, nil
	case "down", "tab":
		if m.mode == modeAdd && len(m.matches) > 0 {
			m.pick = (m.pick + 1) % len(m.matches)
		}
		return m, nil
	case "enter":
		return m.submit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refresh()
	return m, cmd
}

// refresh updates suggestions (add) or the live answer (ask) as you type.
func (m *model) refresh() {
	v := strings.TrimSpace(m.input.Value())
	if m.mode == modeAdd {
		m.matches, m.pick = nil, 0
		if v != "" {
			m.matches = places.Search(v, 6)
		}
		return
	}
	m.preview = ""
	if v == "" {
		return
	}
	q, err := query.Parse(strings.Fields(v))
	if err == nil {
		var res query.Result
		if res, err = q.Resolve(m.cfg.Places[0], m.now, m.cfg.Hour12()); err == nil {
			home := m.rows()[0]
			o := view.Options{Now: m.now, Focus: res.Focus, Hour12: m.cfg.Hour12(), Dark: m.dark}
			m.preview = view.Answer(view.Row{Place: res.Origin, Loc: res.OriginLoc}, home, o)
			return
		}
	}
	m.preview = m.st.faint.Render("  " + err.Error())
}

func (m model) submit() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.input.Value())
	if v == "" {
		m.mode = modeNormal
		return m, nil
	}
	if m.mode == modeAdd {
		var match places.Match
		if len(m.matches) > 0 {
			match = m.matches[m.pick]
		} else {
			var err error
			if match, err = places.Resolve(v); err != nil {
				m.setStatus(err.Error(), true)
				return m, nil
			}
		}
		m.dropTemp(match.Place)
		if m.cfg.Add(match.Place) {
			m.save("Added " + match.Place.Name)
		} else {
			m.setStatus(match.Place.Name+" is already saved", false)
		}
		if match.Note != "" {
			m.notes = []string{match.Note}
		}
		m.sel = view.Find(m.rows(), match.Place)
	} else {
		q, err := query.Parse(strings.Fields(v))
		if err == nil {
			err = m.ask(q)
		}
		if err != nil {
			m.setStatus(err.Error(), true)
			return m, nil
		}
	}
	m.mode = modeNormal
	m.input.Blur()
	return m, nil
}

// ask jumps to the question's time and shows its places until esc.
func (m *model) ask(q query.Query) error {
	res, err := q.Resolve(m.cfg.Places[0], m.now, m.cfg.Hour12())
	if err != nil {
		return err
	}
	m.temp, m.notes = res.Rows, res.Notes
	m.follow = !q.HasTime()
	m.focus = res.Focus
	m.origin = nil
	if len(res.Rows) > 0 {
		o := res.Origin
		m.origin = &o
	}
	m.sel = view.Find(m.rows(), res.Origin)
	return nil
}

func (m model) View() tea.View {
	rows := m.rows()
	o := view.Options{
		Now:      m.now,
		Focus:    m.focus,
		Hour12:   m.cfg.Hour12(),
		Width:    m.width,
		Selected: m.sel,
		Dark:     m.dark,
	}

	origin := rows[0]
	if m.origin != nil {
		origin = rows[view.Find(rows, *m.origin)]
	}

	header := "\n" + view.Answer(origin, rows[0], o)
	notes := view.Notes(m.notes, m.width, m.dark)
	footer := m.footer()

	table := view.Table(rows, o)
	// Many places on a short terminal: drop the weekday lines first.
	used := lipgloss.Height(header) + lipgloss.Height(table) + lipgloss.Height(footer) + 2
	if notes != "" {
		used += lipgloss.Height(notes) + 1
	}
	if m.height > 0 && used > m.height {
		o.Compact = true
		table = view.Table(rows, o)
	}

	parts := []string{header, table}
	if notes != "" {
		parts = append(parts, notes)
	}
	parts = append(parts, footer)

	v := tea.NewView(strings.Join(parts, "\n\n"))
	v.AltScreen = true
	v.WindowTitle = "wtd"
	return v
}

func (m model) footer() string {
	var b strings.Builder
	switch m.mode {
	case modeAdd:
		b.WriteString("  " + m.input.View() + "\n")
		b.WriteString(m.suggestions())
	case modeAsk:
		b.WriteString("  " + m.input.View() + "\n")
		if m.preview != "" {
			b.WriteString("\n" + m.preview + "\n")
		}
		b.WriteString("\n" + m.keys([][2]string{{"enter", "jump there"}, {"esc", "cancel"}}))
	default:
		if m.status != "" {
			style := m.st.status
			if m.statusErr {
				style = m.st.err
			}
			b.WriteString(style.Render("  "+m.status) + "\n\n")
		}
		b.WriteString(m.helpView())
	}
	return b.String()
}

func (m model) suggestions() string {
	if len(m.matches) == 0 {
		if strings.TrimSpace(m.input.Value()) != "" {
			return m.st.faint.Render("\n  no matches") + "\n"
		}
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, s := range m.matches {
		loc, err := s.Place.Location()
		if err != nil {
			continue
		}
		_, off := m.now.In(loc).Zone()
		tag, offset := places.Tag(m.now, loc), places.FormatOffset(off)
		if tag == offset {
			tag = ""
		}
		line := fmt.Sprintf("%-24s %-30s %-6s %-9s %s", trunc(s.Place.Name, 24), trunc(s.Place.Sub, 30),
			tag, offset, query.Clock12(m.now.In(loc), m.cfg.Hour12()))
		if i == m.pick {
			b.WriteString(m.st.pick.Render("  › "+line) + "\n")
		} else {
			b.WriteString(m.st.faint.Render("    "+line) + "\n")
		}
	}
	b.WriteString("\n" + m.keys([][2]string{{"↑/↓", "choose"}, {"enter", "add"}, {"esc", "cancel"}}))
	return b.String()
}

func (m model) helpView() string {
	short := [][2]string{
		{"←/→", "hour"}, {"[/]", "day"}, {"n", "now"}, {"/", "ask"}, {"a", "add"},
		{"d", "delete"}, {"H", "home"}, {"t", "12/24h"}, {"?", "more"}, {"q", "quit"},
	}
	if rows := m.rows(); m.sel < len(rows) && rows[m.sel].Temp {
		short = append([][2]string{{"p", "save " + rows[m.sel].Place.Name}, {"esc", "clear"}}, short...)
	}
	if !m.help {
		return m.keys(short)
	}
	full := [][2]string{
		{"←/→ h/l", "move one hour"},
		{"⇧←/⇧→ < >", "move 15 minutes"},
		{"[ ]", "previous / next day"},
		{"n", "back to now (esc clears a question)"},
		{"↑/↓ k/j", "select a place"},
		{"⇧↑/⇧↓ K/J", "reorder the selected place"},
		{"/", "ask: mst 09:00, 9am tokyo, tomorrow 15:30 sydney"},
		{"a", "add a place"},
		{"p", "save a place shown by a question"},
		{"d", "delete the selected place"},
		{"H", "make the selected place home"},
		{"t", "toggle 12/24-hour clock"},
		{"q", "quit"},
	}
	var b strings.Builder
	for _, kv := range full {
		b.WriteString("  " + m.st.key.Render(fmt.Sprintf("%-11s", kv[0])) + " " + m.st.faint.Render(kv[1]) + "\n")
	}
	return b.String()
}

func (m model) keys(pairs [][2]string) string {
	var parts []string
	for _, kv := range pairs {
		parts = append(parts, m.st.key.Render(kv[0])+" "+m.st.faint.Render(kv[1]))
	}
	line := strings.Join(parts, m.st.faint.Render(" · "))
	if m.width > 0 {
		line = lipgloss.Wrap(line, m.width-4, " ")
	}
	return "  " + strings.ReplaceAll(line, "\n", "\n  ") + "\n"
}

type styles struct {
	key, faint, pick, status, err lipgloss.Style
}

func newStyles(dark bool) styles {
	ld := lipgloss.LightDark(dark)
	c := lipgloss.Color
	return styles{
		key:    lipgloss.NewStyle().Bold(true).Foreground(ld(c("#4A6FB5"), c("#8FB3F0"))),
		faint:  lipgloss.NewStyle().Foreground(ld(c("#8A90A0"), c("#6B7389"))),
		pick:   lipgloss.NewStyle().Bold(true).Foreground(ld(c("#E0457B"), c("#F25D94"))),
		status: lipgloss.NewStyle().Foreground(ld(c("#2E7D4F"), c("#98C379"))),
		err:    lipgloss.NewStyle().Foreground(ld(c("#C0392B"), c("#F2777A"))),
	}
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
