package main

import (
	"context"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/uiapi"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// WailsApp is the Wave-1 backend entrypoint placeholder for future Wails bindings.
// It intentionally reuses the existing command envelope to preserve CLI parity.
type WailsApp struct {
	PipeName string
	service  *uiapi.Service
	ctx      context.Context
}

func NewWailsApp(pipeName string) *WailsApp {
	return &WailsApp{
		PipeName: pipeName,
		service:  uiapi.NewService(pipeName),
	}
}

func (a *WailsApp) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *WailsApp) SelectDirectory(title string) string {
	if a.ctx == nil {
		return ""
	}
	// runtime.OpenDirectoryDialog available from wails runtime
	dir, _ := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
	return dir
}

func (a *WailsApp) SelectFile(title string, filters string) string {
	if a.ctx == nil {
		return ""
	}
	
	options := wailsruntime.OpenDialogOptions{
		Title: title,
	}
	
	if filters != "" {
		options.Filters = []wailsruntime.FileFilter{
			{DisplayName: filters, Pattern: filters},
		}
	}
	
	file, _ := wailsruntime.OpenFileDialog(a.ctx, options)
	return file
}

func (a *WailsApp) AppInfo() map[string]string {
	return map[string]string{
		"name":      "Lightroom Sync",
		"version":   Version,
		"pipe_name": a.PipeName,
		"runtime":   uiRuntimeWails,
		"server_ts": time.Now().Format(time.RFC3339),
	}
}

func (a *WailsApp) Ping() uiapi.ActionEnvelope {
	return a.service.Ping()
}

func (a *WailsApp) ExecuteAction(action, payload string) uiapi.ActionEnvelope {
	return a.service.ExecuteAction(action, payload)
}
