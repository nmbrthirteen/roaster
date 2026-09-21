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
	if cfg.addr != ":9000" || cfg.parallel != 4 || cfg.writer != "anthropic" {
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
	if cfg.writer != "" {
		t.Errorf("with no key the verdict should come from the numbers, got %q", cfg.writer)
	}
}

func TestOpenAIIsUsedWhenItsKeyIsThere(t *testing.T) {
	base := map[string]string{"GITHUB_TOKEN": "x", "TERMINAL_TOKENS": "a-terminal-token-long-enough-1"}

	with := func(extra map[string]string) settings {
		t.Helper()
		vars := map[string]string{}
		for k, v := range base {
			vars[k] = v
		}
		for k, v := range extra {
			vars[k] = v
		}
		cfg, err := fromEnv(env(vars))
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	if cfg := with(map[string]string{"OPENAI_API_KEY": "k"}); cfg.writer != "openai" {
		t.Errorf("OpenAI key alone: got %q", cfg.writer)
	}
	if cfg := with(map[string]string{"OPENAI_API_KEY": "k", "ANTHROPIC_API_KEY": "a"}); cfg.writer != "openai" {
		t.Errorf("both keys: OpenAI should win, got %q", cfg.writer)
	}
	if cfg := with(map[string]string{"OPENAI_API_KEY": "k", "OPENAI_MODEL": "gpt-5.6-luna"}); cfg.model != "gpt-5.6-luna" {
		t.Errorf("the model override should be read, got %q", cfg.model)
	}
}
