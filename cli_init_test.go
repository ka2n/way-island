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
	if !strings.Contains(string(claudeSettings), " hook --claude") {
		t.Fatalf("claude settings missing explicit source flag: %s", claudeSettings)
	}
	if !strings.Contains(string(claudeSettings), "\"async\": true") {
		t.Fatalf("claude settings missing async flag: %s", claudeSettings)
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
	if !strings.Contains(string(codexHooks), " hook --codex") {
		t.Fatalf("codex hooks missing explicit source flag: %s", codexHooks)
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

	codexHooks, err := os.ReadFile(filepath.Join(tmp, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read codex hooks: %v", err)
	}
	if !strings.Contains(string(codexHooks), " hook --codex") {
		t.Fatalf("codex hooks missing explicit source flag: %s", codexHooks)
	}
}

func TestRunInitDebugEmbedsDebugEnvInHookCommands(t *testing.T) {
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
	exitCode := run([]string{"init", "--local", "--debug"}, nil, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	claudeSettings, err := os.ReadFile(filepath.Join(tmp, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read claude settings: %v", err)
	}
	if !strings.Contains(string(claudeSettings), "WAYISLAND_DEBUG=1 ") {
		t.Fatalf("claude settings missing debug env: %s", claudeSettings)
	}

	codexHooks, err := os.ReadFile(filepath.Join(tmp, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read codex hooks: %v", err)
	}
	if !strings.Contains(string(codexHooks), "WAYISLAND_DEBUG=1 ") {
		t.Fatalf("codex hooks missing debug env: %s", codexHooks)
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

func TestEnsureClaudeHookEntryTreatsExistingWayIslandHookAsManaged(t *testing.T) {
	existing := []claudeHookMatcher{
		{
			Matcher: "",
			Hooks: []claudeHookEntry{
				{Type: "command", Command: "/old/path/way-island hook"},
			},
		},
	}

	got := ensureClaudeHookEntry(existing, "/new/path/way-island hook --claude")
	if len(got) != 1 {
		t.Fatalf("expected no duplicate matcher, got %d", len(got))
	}
	if len(got[0].Hooks) != 1 {
		t.Fatalf("expected no duplicate hook entry, got %d", len(got[0].Hooks))
	}
	if got[0].Hooks[0].Command != "/new/path/way-island hook --claude" {
		t.Fatalf("expected managed hook to be updated, got %q", got[0].Hooks[0].Command)
	}
	if !got[0].Hooks[0].Async {
		t.Fatalf("expected managed hook to be async")
	}
}

func TestEnsureCodexHookEntryTreatsExistingWayIslandHookAsManaged(t *testing.T) {
	existing := []codexHookMatcher{
		{
			Hooks: []codexHookEntry{
				{Type: "command", Command: "/old/path/way-island hook"},
			},
		},
	}

	got := ensureCodexHookEntry(existing, "/new/path/way-island hook --codex")
	if len(got) != 1 {
		t.Fatalf("expected no duplicate matcher, got %d", len(got))
	}
	if len(got[0].Hooks) != 1 {
		t.Fatalf("expected no duplicate hook entry, got %d", len(got[0].Hooks))
	}
	if got[0].Hooks[0].Command != "/new/path/way-island hook --codex" {
		t.Fatalf("expected managed hook to be updated, got %q", got[0].Hooks[0].Command)
	}
}

func TestEnsureClaudeHookEntryCollapsesDuplicateManagedHooks(t *testing.T) {
	existing := []claudeHookMatcher{
		{
			Matcher: "",
			Hooks: []claudeHookEntry{
				{Type: "command", Command: "/old/path/way-island hook"},
				{Type: "command", Command: "echo keep"},
			},
		},
		{
			Matcher: "",
			Hooks: []claudeHookEntry{
				{Type: "command", Command: "/other/path/way-island hook --claude"},
			},
		},
	}

	got := ensureClaudeHookEntry(existing, "/new/path/way-island hook --claude")
	if len(got) != 2 {
		t.Fatalf("expected matcher count to be preserved, got %d", len(got))
	}
	if len(got[0].Hooks) != 2 {
		t.Fatalf("expected first matcher to keep one managed hook plus unrelated hook, got %d", len(got[0].Hooks))
	}
	if got[0].Hooks[0].Command != "echo keep" && got[0].Hooks[1].Command != "echo keep" {
		t.Fatalf("expected unrelated hook to be preserved: %#v", got[0].Hooks)
	}
	if countManagedClaudeHooks(got) != 1 {
		t.Fatalf("expected exactly one managed hook after normalization, got %#v", got)
	}
}

func TestEnsureCodexHookEntryCollapsesDuplicateManagedHooks(t *testing.T) {
	existing := []codexHookMatcher{
		{
			Hooks: []codexHookEntry{
				{Type: "command", Command: "/old/path/way-island hook"},
				{Type: "command", Command: "echo keep"},
			},
		},
		{
			Hooks: []codexHookEntry{
				{Type: "command", Command: "/other/path/way-island hook --codex"},
			},
		},
	}

	got := ensureCodexHookEntry(existing, "/new/path/way-island hook --codex")
	if len(got) != 2 {
		t.Fatalf("expected matcher count to be preserved, got %d", len(got))
	}
	if len(got[0].Hooks) != 2 {
		t.Fatalf("expected first matcher to keep one managed hook plus unrelated hook, got %d", len(got[0].Hooks))
	}
	if got[0].Hooks[0].Command != "echo keep" && got[0].Hooks[1].Command != "echo keep" {
		t.Fatalf("expected unrelated hook to be preserved: %#v", got[0].Hooks)
	}
	if countManagedCodexHooks(got) != 1 {
		t.Fatalf("expected exactly one managed hook after normalization, got %#v", got)
	}
}

func countManagedClaudeHooks(matchers []claudeHookMatcher) int {
	count := 0
	for _, matcher := range matchers {
		for _, hook := range matcher.Hooks {
			if isManagedWayIslandHookCommand(hook.Command) {
				count++
			}
		}
	}
	return count
}

func countManagedCodexHooks(matchers []codexHookMatcher) int {
	count := 0
	for _, matcher := range matchers {
		for _, hook := range matcher.Hooks {
			if isManagedWayIslandHookCommand(hook.Command) {
				count++
			}
		}
	}
	return count
}

func TestBuildHookCommand(t *testing.T) {
	if got := buildHookCommand("/tmp/way-island hook --claude", false); got != "/tmp/way-island hook --claude" {
		t.Fatalf("unexpected command without debug: %q", got)
	}
	if got := buildHookCommand("/tmp/way-island hook --claude", true); got != "WAYISLAND_DEBUG=1 /tmp/way-island hook --claude" {
		t.Fatalf("unexpected command with debug: %q", got)
	}
}
