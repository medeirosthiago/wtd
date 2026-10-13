// Package config stores the saved places; the first one is home.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medeirosthiago/wtd/internal/places"
)

type Config struct {
	Places []places.Place `json:"places"`          // Places[0] is home
	Clock  string         `json:"clock"`           // "12h" or "24h"
	Theme  string         `json:"theme,omitempty"` // "auto", "light" or "dark"
}

// Path honours $WTD_CONFIG and $XDG_CONFIG_HOME.
func Path() string {
	if p := os.Getenv("WTD_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "wtd", "config.json")
}

// Load reads the config, starting with the system time zone as home when
// there is none yet.
func Load() (*Config, error) {
	c := &Config{Clock: "12h"}
	data, err := os.ReadFile(Path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("%s: %w", Path(), err)
		}
	}
	if len(c.Places) == 0 {
		c.Places = []places.Place{places.PlaceForZone(places.LocalZone())}
	}
	for _, p := range c.Places {
		if _, err := p.Location(); err != nil {
			return nil, fmt.Errorf("%s: place %q: %w", Path(), p.Name, err)
		}
	}
	return c, nil
}

func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (c *Config) Hour12() bool { return c.Clock != "24h" }

// Index finds a saved place by 1-based position, name or time zone.
func (c *Config) Index(q string) int {
	var n int
	if _, err := fmt.Sscanf(q, "%d", &n); err == nil && fmt.Sprint(n) == q {
		if n >= 1 && n <= len(c.Places) {
			return n - 1
		}
		return -1
	}
	nq := places.Normalize(q)
	for i, p := range c.Places {
		if places.Normalize(p.Name) == nq || strings.EqualFold(p.TZ, q) {
			return i
		}
	}
	return -1
}

// Has reports whether an equivalent place is already saved.
func (c *Config) Has(p places.Place) int {
	for i, s := range c.Places {
		if s.Name == p.Name && s.TZ == p.TZ {
			return i
		}
	}
	return -1
}

// Add appends p unless it is already saved; it reports whether it was added.
func (c *Config) Add(p places.Place) bool {
	if c.Has(p) >= 0 {
		return false
	}
	c.Places = append(c.Places, p)
	return true
}

// SetHome moves p to the front, adding it if needed. The old home stays.
func (c *Config) SetHome(p places.Place) {
	if i := c.Has(p); i >= 0 {
		c.Places = append(c.Places[:i], c.Places[i+1:]...)
	}
	c.Places = append([]places.Place{p}, c.Places...)
}

func (c *Config) Remove(i int) error {
	if i < 0 || i >= len(c.Places) {
		return errors.New("no such place")
	}
	if len(c.Places) == 1 {
		return errors.New("can't remove the only place; set another home first")
	}
	c.Places = append(c.Places[:i], c.Places[i+1:]...)
	return nil
}

// Move shifts the place at i by delta positions, staying in range.
func (c *Config) Move(i, delta int) int {
	j := min(max(i+delta, 0), len(c.Places)-1)
	if i < 0 || i >= len(c.Places) || i == j {
		return i
	}
	p := c.Places[i]
	c.Places = append(c.Places[:i], c.Places[i+1:]...)
	c.Places = append(c.Places[:j], append([]places.Place{p}, c.Places[j:]...)...)
	return j
}
