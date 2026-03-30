package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func launchWailsRuntime(pipeName string) error {
	projectRoot, err := resolveWailsProjectRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command("wails", "dev")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(
		os.Environ(),
		"LIGHTROOMSYNC_PIPE="+pipeName,
		"LIGHTROOMSYNC_UI_VERSION="+Version,
	)

	log.Printf("[INFO] Launching Wails runtime (dev mode) from %s", projectRoot)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run wails dev: %w", err)
	}

	return nil
}

func resolveWailsProjectRoot() (string, error) {
	candidates := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		candidates = append(candidates, cwd)
	}
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		candidates = append(candidates, filepath.Dir(exePath))
	}

	checked := make(map[string]struct{}, 16)
	for _, candidate := range candidates {
		root := filepath.Clean(candidate)
		for {
			if _, seen := checked[root]; !seen {
				checked[root] = struct{}{}
				if fileExists(filepath.Join(root, "wails.json")) {
					return root, nil
				}
			}

			parent := filepath.Dir(root)
			if parent == root {
				break
			}
			root = parent
		}
	}

	return "", fmt.Errorf("wails.json not found from current working directory or executable path")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
