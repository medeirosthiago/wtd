package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medeirosthiago/wtd/internal/places"
)

func names(c *Config) string {
	var n []string
	for _, p := range c.Places {
		n = append(n, p.Name)
	}
	return strings.Join(n, ",")
}

func pl(name string) places.Place { return places.Place{Name: name, TZ: "UTC"} }

func TestEditing(t *testing.T) {
	c := &Config{Places: []places.Place{pl("A"), pl("B"), pl("C"), pl("D")}}

	if c.Add(pl("B")) {
		t.Error("Add accepted a duplicate")
	}
	c.SetHome(pl("C"))
	if got := names(c); got != "C,A,B,D" {
		t.Errorf("SetHome existing: %s", got)
	}
	c.SetHome(pl("E"))
	if got := names(c); got != "E,C,A,B,D" {
		t.Errorf("SetHome new: %s", got)
	}
	if j := c.Move(4, -2); j != 2 || names(c) != "E,C,D,A,B" {
		t.Errorf("Move: %d %s", j, names(c))
	}
	if err := c.Remove(0); err != nil || names(c) != "C,D,A,B" {
		t.Errorf("Remove home: %v %s", err, names(c))
	}
	if i := c.Index("2"); i != 1 {
		t.Errorf("Index(2) = %d", i)
	}
	if i := c.Index("a"); i != 2 {
		t.Errorf("Index(a) = %d", i)
	}
	if i := c.Index("9"); i != -1 {
		t.Errorf("Index(9) = %d", i)
	}
	only := &Config{Places: []places.Place{pl("A")}}
	if err := only.Remove(0); err == nil {
		t.Error("removed the only place")
	}
}

func TestSaveLoad(t *testing.T) {
	t.Setenv("WTD_CONFIG", filepath.Join(t.TempDir(), "wtd", "config.json"))
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Places) != 1 || c.Clock != "12h" {
		t.Fatalf("fresh config = %+v, want the local zone as home", c)
	}
	c.Add(places.Place{Name: "Taipei", Sub: "Taiwan", TZ: "Asia/Taipei"})
	c.Clock = "24h"
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if names(c2) != names(c) || c2.Clock != "24h" {
		t.Errorf("reloaded %s %s, want %s 24h", names(c2), c2.Clock, names(c))
	}
}
