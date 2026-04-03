package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitConfiguresClaudeAndCodexLocallyByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stderr bytes.Buffer
	exitCode := run([]string{"init", "--local"}, nil, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	claudeSettings, err := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read claude settings: %v", err)
	}
	for _, eventName := range claudeHookEventNames {
		if !strings.Contains(string(claudeSettings), eventName) {
			t.Fatalf("claude settings missing event %q: %s", eventName, claudeSettings)
		}
	}

	codexHooks, err := os.ReadFile(filepath.Join(tmp, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read codex hooks: %v", err)
	}
	for _, eventName := range codexHookEventNames {
		if !strings.Contains(string(codexHooks), eventName) {
			t.Fatalf("codex hooks missing event %q: %s", eventName, codexHooks)
		}
	}

	codexConfig, err := os.ReadFile(filepath.Join(tmp, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(codexConfig), "codex_hooks = true") {
		t.Fatalf("codex config missing feature flag: %s", codexConfig)
	}
}

func TestRunInitCanConfigureOnlyCodex(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stderr bytes.Buffer
	exitCode := run([]string{"init", "--local", "--codex"}, nil, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(tmp, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("claude settings should not exist, err=%v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".codex", "hooks.json")); err != nil {
		t.Fatalf("codex hooks should exist: %v", err)
	}
}

func TestMergeCodexHooksFeature(t *testing.T) {
	t.Run("appends missing features section", func(t *testing.T) {
		got := string(mergeCodexHooksFeature([]byte("model = \"gpt-5\"\n")))
		if got != "model = \"gpt-5\"\n\n[features]\ncodex_hooks = true\n" {
			t.Fatalf("unexpected config: %q", got)
		}
	})

	t.Run("updates existing assignment", func(t *testing.T) {
		got := string(mergeCodexHooksFeature([]byte("[features]\ncodex_hooks = false\n")))
		if got != "[features]\ncodex_hooks = true\n" {
			t.Fatalf("unexpected config: %q", got)
		}
	})

	t.Run("preserves surrounding sections", func(t *testing.T) {
		got := string(mergeCodexHooksFeature([]byte("[features]\nfoo = 1\n[profiles.default]\nmodel = \"gpt-5\"\n")))
		want := "[features]\nfoo = 1\ncodex_hooks = true\n[profiles.default]\nmodel = \"gpt-5\"\n"
		if got != want {
			t.Fatalf("unexpected config:\n%s", got)
		}
	})
}
