package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var claudeHookEventNames = []string{"PreToolUse", "PostToolUse", "Notification", "Stop"}

var codexHookEventNames = []string{"SessionStart", "SessionEnd", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}

type claudeHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
}

type claudeHookMatcher struct {
	Matcher string            `json:"matcher"`
	Hooks   []claudeHookEntry `json:"hooks"`
}

type codexHookFile struct {
	Hooks map[string][]codexHookMatcher `json:"hooks"`
}

type codexHookMatcher struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []codexHookEntry `json:"hooks"`
}

type codexHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type initTarget struct {
	Name string
	Run  bool
}

func runInit(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	local := fs.Bool("local", false, "Write repo-local config instead of global config")
	claude := fs.Bool("claude", false, "Configure Claude Code hooks")
	codex := fs.Bool("codex", false, "Configure Codex hooks")
	debug := fs.Bool("debug", false, "Enable WAYISLAND_DEBUG=1 for installed hook commands")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "usage: way-island init [--local] [--claude] [--codex] [--debug]\n")
		return 2
	}

	targets := resolveInitTargets(*claude, *codex)

	execPath, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve executable path: %v\n", err)
		return 1
	}

	var configured []string
	for _, target := range targets {
		if !target.Run {
			continue
		}

		var configureErr error
		switch target.Name {
		case "claude":
			configureErr = configureClaude(*local, execPath, *debug)
		case "codex":
			configureErr = configureCodex(*local, execPath, *debug)
		default:
			configureErr = fmt.Errorf("unsupported target %q", target.Name)
		}
		if configureErr != nil {
			_, _ = fmt.Fprintf(stderr, "failed to configure %s hooks: %v\n", target.Name, configureErr)
			return 1
		}

		configured = append(configured, target.Name)
	}

	fmt.Printf("Configured hooks for %s\n", strings.Join(configured, ", "))
	fmt.Printf("Run way-island in your River init script to start the daemon.\n")

	return 0
}

func resolveInitTargets(claude, codex bool) []initTarget {
	if !claude && !codex {
		claude = true
		codex = true
	}

	return []initTarget{
		{Name: "claude", Run: claude},
		{Name: "codex", Run: codex},
	}
}

func configureClaude(local bool, execPath string, debug bool) error {
	settingsPath, err := resolveClaudeSettingsPath(local)
	if err != nil {
		return fmt.Errorf("resolve settings path: %w", err)
	}

	settings, err := loadJSONFile(settingsPath)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	command := buildHookCommand(execPath+" hook --claude", debug)
	for _, eventName := range claudeHookEventNames {
		hooks[eventName] = ensureClaudeHookEntry(hooks[eventName], command)
	}

	settings["hooks"] = hooks

	if err := saveJSONFile(settingsPath, settings); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}

	return nil
}

func configureCodex(local bool, execPath string, debug bool) error {
	hooksPath, configPath, err := resolveCodexPaths(local)
	if err != nil {
		return err
	}

	hooksFile, err := loadCodexHooksFile(hooksPath)
	if err != nil {
		return fmt.Errorf("load hooks file: %w", err)
	}

	command := buildHookCommand(execPath+" hook --codex", debug)
	for _, eventName := range codexHookEventNames {
		hooksFile.Hooks[eventName] = ensureCodexHookEntry(hooksFile.Hooks[eventName], command)
	}

	if err := saveJSONFile(hooksPath, hooksFile); err != nil {
		return fmt.Errorf("save hooks file: %w", err)
	}

	if err := enableCodexHooksFeature(configPath); err != nil {
		return fmt.Errorf("enable codex hook feature: %w", err)
	}

	return nil
}

func resolveClaudeSettingsPath(local bool) (string, error) {
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

func resolveCodexPaths(local bool) (hooksPath string, configPath string, err error) {
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		return filepath.Join(cwd, ".codex", "hooks.json"), filepath.Join(cwd, ".codex", "config.toml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, ".codex", "hooks.json"), filepath.Join(home, ".codex", "config.toml"), nil
}

func buildHookCommand(command string, debug bool) string {
	if !debug {
		return command
	}
	return "WAYISLAND_DEBUG=1 " + command
}

func ensureClaudeHookEntry(existing any, command string) []claudeHookMatcher {
	matchers := toClaudeHookMatchers(existing)
	foundManaged := false
	managedMatcherIndex := -1

	for i := range matchers {
		filtered := matchers[i].Hooks[:0]
		for _, h := range matchers[i].Hooks {
			if !isManagedWayIslandHookCommand(h.Command) {
				filtered = append(filtered, h)
				continue
			}
			if !foundManaged {
				foundManaged = true
				managedMatcherIndex = i
			}
		}
		matchers[i].Hooks = filtered
	}

	if foundManaged {
		matchers[managedMatcherIndex].Hooks = append(matchers[managedMatcherIndex].Hooks, claudeHookEntry{
			Type:    "command",
			Command: command,
			Async:   true,
		})
		return matchers
	}

	return append(matchers, claudeHookMatcher{
		Matcher: "",
		Hooks:   []claudeHookEntry{{Type: "command", Command: command, Async: true}},
	})
}

func ensureCodexHookEntry(existing []codexHookMatcher, command string) []codexHookMatcher {
	foundManaged := false
	managedMatcherIndex := -1

	for i := range existing {
		filtered := existing[i].Hooks[:0]
		for _, h := range existing[i].Hooks {
			if !isManagedWayIslandHookCommand(h.Command) {
				filtered = append(filtered, h)
				continue
			}
			if !foundManaged {
				foundManaged = true
				managedMatcherIndex = i
			}
		}
		existing[i].Hooks = filtered
	}

	if foundManaged {
		existing[managedMatcherIndex].Hooks = append(existing[managedMatcherIndex].Hooks, codexHookEntry{
			Type:    "command",
			Command: command,
		})
		return existing
	}

	return append(existing, codexHookMatcher{
		Hooks: []codexHookEntry{{Type: "command", Command: command}},
	})
}

func isManagedWayIslandHookCommand(command string) bool {
	return strings.Contains(command, "way-island hook")
}

func toClaudeHookMatchers(v any) []claudeHookMatcher {
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

func loadJSONFile(path string) (map[string]any, error) {
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

func saveJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func loadCodexHooksFile(path string) (codexHookFile, error) {
	settings, err := loadJSONFile(path)
	if err != nil {
		return codexHookFile{}, err
	}

	file := codexHookFile{Hooks: make(map[string][]codexHookMatcher)}
	if len(settings) == 0 {
		return file, nil
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return codexHookFile{}, err
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return codexHookFile{}, err
	}
	if file.Hooks == nil {
		file.Hooks = make(map[string][]codexHookMatcher)
	}
	return file, nil
}

func enableCodexHooksFeature(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	updated := mergeCodexHooksFeature(data)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, updated, 0o644)
}

func mergeCodexHooksFeature(existing []byte) []byte {
	if len(existing) == 0 {
		return []byte("[features]\ncodex_hooks = true\n")
	}

	lines := strings.Split(string(existing), "\n")
	featuresIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "[features]" {
			featuresIndex = i
			break
		}
	}

	if featuresIndex == -1 {
		body := strings.TrimRight(string(existing), "\n")
		if body == "" {
			return []byte("[features]\ncodex_hooks = true\n")
		}
		return []byte(body + "\n\n[features]\ncodex_hooks = true\n")
	}

	for i := featuresIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			lines = insertString(lines, i, "codex_hooks = true")
			return normalizeTrailingNewline(lines)
		}
		if isCodexHooksAssignment(trimmed) {
			lines[i] = replaceCodexHooksAssignment(lines[i])
			return normalizeTrailingNewline(lines)
		}
	}

	lines = append(lines, "codex_hooks = true")
	return normalizeTrailingNewline(lines)
}

func isCodexHooksAssignment(line string) bool {
	return strings.HasPrefix(line, "codex_hooks") && strings.Contains(line, "=")
}

func replaceCodexHooksAssignment(line string) string {
	leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	return leading + "codex_hooks = true"
}

func insertString(values []string, index int, value string) []string {
	values = append(values, "")
	copy(values[index+1:], values[index:])
	values[index] = value
	return values
}

func normalizeTrailingNewline(lines []string) []byte {
	text := strings.Join(lines, "\n")
	text = strings.TrimRight(text, "\n") + "\n"
	return []byte(text)
}
