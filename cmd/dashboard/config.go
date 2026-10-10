package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is read from config.toml in the plugin's config directory
// (HERDR_PLUGIN_CONFIG_DIR, or $AGENT_DASHBOARD_CONFIG for a one-off path).
// Every key is optional; see config.example.toml.
type Config struct {
	DefaultView    string   `toml:"default_view"` // tree | graph | grid | table | fan
	PollSeconds    int      `toml:"poll_seconds"` // agent list, branch, model, phase
	PRRefreshSecs  int      `toml:"pr_refresh_seconds"`
	DetailRefresh  int      `toml:"detail_refresh_seconds"`
	DoubleClickMs  int      `toml:"double_click_ms"`
	ExpandOnClick  bool     `toml:"expand_on_click"` // single click toggles the drawer
	BeadPattern    string   `toml:"bead_pattern"`    // agent name -> bead id
	BeadsDir       string   `toml:"beads_dir"`       // BEADS_DIR for bd, when the pane does not inherit it
	ExtraPath      []string `toml:"extra_path"`      // appended to PATH for bd / gh / git
	OwnerMode      string   `toml:"owner_mode"`      // primary | none
	SessionTailKB  int      `toml:"session_tail_kb"` // how much of each session log to read
	CardMinWidth   int      `toml:"card_min_width"`
	DescriptionMax int      `toml:"description_lines"`
	SaidChars      int      `toml:"said_chars"`
	RecentTools    int      `toml:"recent_tools"`
	CloseOnJump    bool     `toml:"close_on_jump"` // double-click/enter closes the dashboard after focusing the pane
	Animate        bool     `toml:"animate"`       // comet on working cards, amber flash on blocked ones
}

var cfg = defaultConfig()

func defaultConfig() Config {
	return Config{
		DefaultView: "graph", PollSeconds: 2, PRRefreshSecs: 30, DetailRefresh: 15,
		DoubleClickMs: 400, ExpandOnClick: true, BeadPattern: `^[a-z]+-[a-z0-9]+$`,
		ExtraPath: []string{"~/.local/bin", "~/go/bin", "/opt/homebrew/bin", "/usr/local/bin"},
		OwnerMode: "primary", SessionTailKB: 1024, CardMinWidth: 40,
		DescriptionMax: 18, SaidChars: 500, RecentTools: 6, Animate: true,
	}
}

var beadRe = regexp.MustCompile(defaultConfig().BeadPattern)

func expandHome(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// loadConfig reads path (if it exists) over the defaults, then fixes up bad values.
func loadConfig(path string) {
	cfg = defaultConfig()
	if path != "" {
		toml.DecodeFile(path, &cfg) // a missing or broken file leaves the defaults
	}
	if re, err := regexp.Compile(cfg.BeadPattern); err == nil {
		beadRe = re
	} else {
		beadRe = regexp.MustCompile(defaultConfig().BeadPattern)
	}
	for _, p := range [...]*int{&cfg.PollSeconds, &cfg.PRRefreshSecs, &cfg.DetailRefresh, &cfg.DoubleClickMs,
		&cfg.SessionTailKB, &cfg.CardMinWidth, &cfg.DescriptionMax, &cfg.SaidChars, &cfg.RecentTools} {
		if *p < 1 {
			*p = 1
		}
	}
	cfg.CardMinWidth = max(cfg.CardMinWidth, 30)
	cfg.BeadsDir = expandHome(cfg.BeadsDir)
}

func configPath() string {
	if p := os.Getenv("AGENT_DASHBOARD_CONFIG"); p != "" {
		return p
	}
	if d := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "config.toml")
	}
	return ""
}

func sec(n int) time.Duration { return time.Duration(n) * time.Second }

func (c Config) viewIndex() int {
	for i, m := range modes {
		if m == c.DefaultView {
			return i
		}
	}
	return 1 // graph
}
