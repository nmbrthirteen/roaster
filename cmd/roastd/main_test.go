package main

import (
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestSettingsRefuseToStartUnsafe(t *testing.T) {
	good := "a-terminal-token-long-enough-1"
	for name, vars := range map[string]map[string]string{
		"no GitHub token":   {"TERMINAL_TOKENS": good},
		"no terminals":      {"GITHUB_TOKEN": "x"},
		"a typed password":  {"GITHUB_TOKEN": "x", "TERMINAL_TOKENS": good + ",hunter2"},
		"a nonsense number": {"GITHUB_TOKEN": "x", "TERMINAL_TOKENS": good, "ROAST_PARALLEL": "lots"},
	} {
		if _, err := fromEnv(env(vars)); err == nil {
			t.Errorf("%s: the service should refuse to start", name)
		}
	}
}

func TestSettingsReadTheEnvironment(t *testing.T) {
	cfg, err := fromEnv(env(map[string]string{
		"GITHUB_TOKEN":      "x",
		"ANTHROPIC_API_KEY": "y",
		"TERMINAL_TOKENS":   " a-terminal-token-long-enough-1 , a-terminal-token-long-enough-2 ,",
		"ROAST_OPT_OUT":     "@Somebody, another",
		"PORT":              "9000",
		"ROAST_PARALLEL":    "4",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != ":9000" || cfg.parallel != 4 || !cfg.model {
		t.Errorf("got %+v", cfg)
	}
	if len(cfg.tokens) != 2 || strings.HasPrefix(cfg.tokens[0], " ") {
		t.Errorf("tokens should be trimmed and blanks dropped, got %q", cfg.tokens)
	}
	if !cfg.optOut["somebody"] || !cfg.optOut["another"] {
		t.Errorf("opt-outs should be folded the way the cache folds handles, got %v", cfg.optOut)
	}
}

func TestNoModelKeyStillRuns(t *testing.T) {
	cfg, err := fromEnv(env(map[string]string{"GITHUB_TOKEN": "x", "TERMINAL_TOKENS": "a-terminal-token-long-enough-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.model {
		t.Errorf("with no key the verdict should come from the numbers")
	}
}
