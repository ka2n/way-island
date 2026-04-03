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
