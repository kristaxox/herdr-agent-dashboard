package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte("default_view = \"grid\"\npoll_seconds = 5\nbead_pattern = '^bd-[0-9]+$'\nowner_mode = \"none\"\nrecent_tools = 0\n"), 0o600)
	loadConfig(p)
	if cfg.viewIndex() != 2 || cfg.PollSeconds != 5 || cfg.OwnerMode != "none" || cfg.DescriptionMax != 18 {
		t.Errorf("overrides/defaults wrong: %+v", cfg)
	}
	if !beadRe.MatchString("bd-12") || beadRe.MatchString("abc-123") {
		t.Error("bead_pattern not applied")
	}
	if cfg.RecentTools != 1 {
		t.Errorf("zero should be clamped, got %d", cfg.RecentTools)
	}
	os.WriteFile(p, []byte("bead_pattern = '['\ndefault_view = \"nope\"\n"), 0o600)
	loadConfig(p)
	if !beadRe.MatchString("abc-123") || cfg.viewIndex() != 1 {
		t.Error("bad regex / view should fall back to defaults")
	}
	loadConfig("") // leave defaults for other tests
}
