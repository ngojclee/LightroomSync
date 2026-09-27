//go:build windows

package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	optionswindows "github.com/wailsapp/wails/v2/pkg/options/windows"
)

func applyWailsPlatformOptions(appOptions *options.App) {
	appOptions.Windows = &optionswindows.Options{
		DisablePinchZoom: true,
		// OS-adaptive backdrop — Mica on Windows 11. Graceful no-op on 10.
		BackdropType: optionswindows.Auto,
		// Frameless + translucent: the .lrs-shell wrapper paints rounded
		// corners while the OS backdrop shows through the transparent region.
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
	}
	appOptions.BackgroundColour = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
}
