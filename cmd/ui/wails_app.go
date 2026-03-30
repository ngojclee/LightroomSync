package main

import (
	"time"

	"github.com/ngojclee/lightroom-sync/internal/uiapi"
)

// WailsApp is the Wave-1 backend entrypoint placeholder for future Wails bindings.
// It intentionally reuses the existing command envelope to preserve CLI parity.
type WailsApp struct {
	PipeName string
	service  *uiapi.Service
}

func NewWailsApp(pipeName string) *WailsApp {
	return &WailsApp{
		PipeName: pipeName,
		service:  uiapi.NewService(pipeName),
	}
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
