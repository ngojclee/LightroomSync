//go:build wails && !windows

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func applyWailsPlatformOptions(_ *options.App) {}
