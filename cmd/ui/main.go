package main

import (
	"context"
	"log"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

var Version = "dev"

func main() {
	// TODO(phase1.3): Wails window + IPC client to Agent
	log.Printf("[INFO] LightroomSync UI %s", Version)

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 2200*time.Millisecond)
	defer cancelWait()
	err := ipc.WaitForAgent(waitCtx, ipc.PipeName, 150*time.Millisecond)
	if err != nil {
		log.Printf("[WARN] Agent not reachable via IPC: %v", err)
		log.Println("[INFO] UI will continue in placeholder mode (Wails pending Phase 6)")
		return
	}

	// Fetch a quick status snapshot to validate request-response contract.
	statusCtx, cancelStatus := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelStatus()

	resp, err := ipc.Call(statusCtx, ipc.PipeName, ipc.Request{Command: ipc.CmdGetStatus})
	if err != nil {
		log.Printf("[WARN] Failed to query status from Agent: %v", err)
		return
	}
	log.Printf("[INFO] Connected to Agent. IPC code=%s success=%v", resp.Code, resp.Success)
	if status, ok := resp.Data.(map[string]any); ok {
		if hint, ok := status["migration_hint"].(string); ok && hint != "" {
			log.Printf("[INFO] Migration hint: %s", hint)
		}
	}
	log.Println("[INFO] UI process placeholder — Wails integration pending Phase 6")
}
