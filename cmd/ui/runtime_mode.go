package main

import "strings"

const (
	uiRuntimeHarness = "harness"
	uiRuntimeWails   = "wails"
)

func normalizeRuntimeMode(mode string) string {
	trimmed := strings.ToLower(strings.TrimSpace(mode))
	if trimmed == "" {
		return uiRuntimeHarness
	}
	return trimmed
}
