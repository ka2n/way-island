package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUserStyleCSSPathUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := resolveUserStyleCSSPath()
	if err != nil {
		t.Fatalf("resolve user style CSS path: %v", err)
	}

	want := filepath.Join(tmp, "way-island", "style.css")
	if got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
}

func TestLoadAppCSSFallsBackToDefaultWhenUserFileMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := loadAppCSS("window.background { background: transparent; }")
	if err != nil {
		t.Fatalf("load app CSS: %v", err)
	}

	want := "window.background { background: transparent; }"
	if got != want {
		t.Fatalf("unexpected CSS: got %q want %q", got, want)
	}
}

func TestLoadAppCSSAppendsUserOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	path := filepath.Join(tmp, "way-island", "style.css")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(".island-pill { border-radius: 24px; }\n"), 0o644); err != nil {
		t.Fatalf("write user CSS: %v", err)
	}

	got, err := loadAppCSS("window.background { background: transparent; }")
	if err != nil {
		t.Fatalf("load app CSS: %v", err)
	}

	want := "window.background { background: transparent; }\n\n.island-pill { border-radius: 24px; }\n"
	if got != want {
		t.Fatalf("unexpected CSS: got %q want %q", got, want)
	}
}

func TestLoadUserStyleCSSReturnsEmptyWhenMissing(t *testing.T) {
	got, err := loadUserStyleCSS(filepath.Join(t.TempDir(), "missing.css"))
	if err != nil {
		t.Fatalf("load user style CSS: %v", err)
	}
	if got != "" {
		t.Fatalf("unexpected CSS: %q", got)
	}
}

func TestMergeAppCSSSkipsSeparatorWhenUserCSSMissing(t *testing.T) {
	got := mergeAppCSS("window.background { background: transparent; }", "")
	want := "window.background { background: transparent; }"
	if got != want {
		t.Fatalf("unexpected CSS: got %q want %q", got, want)
	}
}

func TestResolveUserConfigPathUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got, err := resolveUserConfigPath()
	if err != nil {
		t.Fatalf("resolve user config path: %v", err)
	}

	want := filepath.Join(tmp, "way-island", "config.json")
	if got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
}

func TestLoadAppConfigReturnsZeroWhenMissing(t *testing.T) {
	got, err := loadAppConfigFromPath(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("load app config: %v", err)
	}
	if got.Focus.RetitleWithOSC {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestLoadAppConfigReadsFocusSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{\"focus\":{\"retitle_with_osc\":true}}"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := loadAppConfigFromPath(path)
	if err != nil {
		t.Fatalf("load app config: %v", err)
	}
	if !got.Focus.RetitleWithOSC {
		t.Fatalf("expected retitle_with_osc to be enabled: %#v", got)
	}
}
