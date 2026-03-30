//go:build wails

package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func launchWailsRuntime(pipeName string) error {
	projectRoot, err := resolveWailsProjectRoot()
	if err != nil {
		return err
	}

	assetsPath := filepath.Join(projectRoot, "frontend", "dist")
	assetsFS, err := resolveWailsAssetsFS(assetsPath)
	if err != nil {
		return err
	}

	app := NewWailsApp(pipeName)
	appOptions := &options.App{
		Title:     "Lightroom Sync",
		MinWidth:  980,
		MinHeight: 700,
		Width:     1180,
		Height:    820,
		AssetServer: &assetserver.Options{
			Assets: assetsFS,
		},
		Bind: []interface{}{
			app,
		},
	}
	applyWailsPlatformOptions(appOptions)

	log.Printf("[INFO] Launching embedded Wails runtime from %s", assetsPath)
	if err := wails.Run(appOptions); err != nil {
		return fmt.Errorf("run embedded wails app: %w", err)
	}
	return nil
}

func resolveWailsAssetsFS(assetsPath string) (fs.FS, error) {
	indexPath := filepath.Join(assetsPath, "index.html")
	if !fileExists(indexPath) {
		return nil, fmt.Errorf("missing Wails frontend assets at %s; run `npm --prefix frontend run build` first", assetsPath)
	}
	return os.DirFS(assetsPath), nil
}
