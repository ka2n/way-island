package main

import (
	"errors"
	"os"
	"path/filepath"
)

const userStyleCSSRelativePath = "way-island/style.css"

func loadAppCSS(defaultCSS string) (string, error) {
	path, err := resolveUserStyleCSSPath()
	if err != nil {
		return defaultCSS, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultCSS, nil
		}
		return defaultCSS, err
	}

	if len(data) == 0 {
		return defaultCSS, nil
	}

	return defaultCSS + "\n\n" + string(data), nil
}

func resolveUserStyleCSSPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, userStyleCSSRelativePath), nil
}
