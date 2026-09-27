//go:build windows

package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	optionswindows "github.com/wailsapp/wails/v2/pkg/options/windows"
)

func applyWailsPlatformOptions(appOptions *options.App) {
	appOptions.Windows = &optionswindows.Options{
		DisablePinchZoom: true,
		// OS-adaptive backdrop — Mica on Windows 11 gives the native caption
		// and frame the modern material look (rounded corners come free from
		// DWM for framed windows). Graceful no-op on Windows 10.
		BackdropType: optionswindows.Auto,
	}
}
