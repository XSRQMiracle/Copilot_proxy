package web

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	bgOnce   sync.Once
	bgDir    string
	bgSource string
	bgErr    error
)

// AllowedBackgroundFiles returns the set of filenames that may be served
// via the /background/ route.
func AllowedBackgroundFiles() map[string]bool {
	return map[string]bool{
		"light.png": true,
		"dark.png":  true,
	}
}

// BackgroundDir returns the resolved background directory path and a human-
// readable source label. It searches in order:
//  1. $COPILOT_PROXY_BACKGROUND_DIR
//  2. <cwd>/background
//  3. <exeDir>/background
//
// When the directory is missing a non-nil error is returned — callers should
// treat this as non-fatal and fall back gracefully.
func BackgroundDir() (dir, source string, err error) {
	bgOnce.Do(func() {
		bgDir, bgSource, bgErr = resolveBackgroundDir()
	})
	return bgDir, bgSource, bgErr
}

func resolveBackgroundDir() (string, string, error) {
	if dir := os.Getenv("COPILOT_PROXY_BACKGROUND_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, fmt.Sprintf("$COPILOT_PROXY_BACKGROUND_DIR (%s)", dir), nil
		}
		return "", "", fmt.Errorf("$COPILOT_PROXY_BACKGROUND_DIR=%q: not a directory", dir)
	}

	if cwd, err := os.Getwd(); err == nil {
		dir := filepath.Join(cwd, "background")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, dir, nil
		}
	}

	if exec, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exec), "background")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, dir, nil
		}
	}

	return "", "", fmt.Errorf("background directory not found")
}

// ServeBackgroundPath returns the absolute filesystem path for a background
// image filename. The filename must be in the allowlist returned by
// AllowedBackgroundFiles. Returns an empty string if the file is unknown or
// the background directory is not available.
func ServeBackgroundPath(filename string) string {
	if !AllowedBackgroundFiles()[filename] {
		return ""
	}
	dir, _, err := BackgroundDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, filename)
}
