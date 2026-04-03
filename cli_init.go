package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// hookEventNames are the Claude Code hook event types way-island listens to.
var hookEventNames = []string{"PreToolUse", "PostToolUse", "Notification", "Stop"}

// claudeHookEntry matches the Claude Code settings.json hook entry format.
type claudeHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// claudeHookMatcher matches the Claude Code settings.json hook matcher format.
type claudeHookMatcher struct {
	Matcher string            `json:"matcher"`
	Hooks   []claudeHookEntry `json:"hooks"`
}

func runInit(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	local := fs.Bool("local", false, "Write to .claude/settings.local.json in current directory instead of global settings")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "usage: way-island init [--local]\n")
		return 2
	}

	settingsPath, err := resolveSettingsPath(*local)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve settings path: %v\n", err)
		return 1
	}

	execPath, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve executable path: %v\n", err)
		return 1
	}

	if err := configureHooks(settingsPath, execPath); err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to configure hooks: %v\n", err)
		return 1
	}

	fmt.Printf("Configured hooks in %s\n", settingsPath)
	fmt.Printf("Run way-island in your River init script to start the daemon.\n")

	return 0
}

func resolveSettingsPath(local bool) (string, error) {
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func configureHooks(settingsPath, execPath string) error {
	settings, err := loadSettingsFile(settingsPath)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	command := execPath + " hook"
	for _, eventName := range hookEventNames {
		hooks[eventName] = ensureHookEntry(hooks[eventName], command)
	}

	settings["hooks"] = hooks

	return saveSettingsFile(settingsPath, settings)
}

// ensureHookEntry adds a way-island hook entry if not already present.
func ensureHookEntry(existing any, command string) []claudeHookMatcher {
	matchers := toHookMatchers(existing)

	for _, m := range matchers {
		for _, h := range m.Hooks {
			if h.Command == command {
				return matchers // already configured
			}
		}
	}

	return append(matchers, claudeHookMatcher{
		Matcher: "",
		Hooks:   []claudeHookEntry{{Type: "command", Command: command}},
	})
}

// toHookMatchers converts the raw JSON value to a slice of claudeHookMatcher.
func toHookMatchers(v any) []claudeHookMatcher {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var matchers []claudeHookMatcher
	if err := json.Unmarshal(raw, &matchers); err != nil {
		return nil
	}
	return matchers
}

func loadSettingsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		settings = make(map[string]any)
	}
	return settings, nil
}

func saveSettingsFile(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

