package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

var Version = "dev"

type uiApp struct {
	pipeName string
	server   *http.Server
}

func main() {
	log.Printf("[INFO] LightroomSync UI %s", Version)

	app := &uiApp{
		pipeName: ipc.PipeName,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/api/ping", app.handlePing)
	mux.HandleFunc("/api/status", app.handleStatus)
	mux.HandleFunc("/api/sync-now", app.handleSyncNow)
	mux.HandleFunc("/api/exit", app.handleExit)

	app.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 4 * time.Second,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to start local UI server: %v", err)
	}
	defer ln.Close()

	uiURL := "http://" + ln.Addr().String()

	go func() {
		if serveErr := app.server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("[ERROR] UI server stopped unexpectedly: %v", serveErr)
		}
	}()

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 2200*time.Millisecond)
	defer cancelWait()
	err = ipc.WaitForAgent(waitCtx, ipc.PipeName, 150*time.Millisecond)
	if err != nil {
		log.Printf("[WARN] Agent not reachable at startup: %v", err)
	} else {
		log.Println("[INFO] Agent reachable. Opening test GUI...")
	}

	if err := openBrowser(uiURL); err != nil {
		log.Printf("[WARN] Failed to open browser automatically: %v", err)
		log.Printf("[INFO] Open manually: %s", uiURL)
	}
	log.Printf("[INFO] UI running at %s", uiURL)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("[INFO] Shutting down UI...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShutdown()
	_ = app.server.Shutdown(shutdownCtx)
}

func (a *uiApp) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(uiHTML))
}

func (a *uiApp) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
	defer cancel()

	ok, err := ipc.Ping(ctx, a.pipeName)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"success": false,
			"error":   err.Error(),
			"code":    ipc.CodeAgentOffline,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": ok,
		"code":    ipc.CodeOK,
	})
}

func (a *uiApp) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, a.pipeName, ipc.Request{Command: ipc.CmdGetStatus})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"success": false,
			"error":   err.Error(),
			"code":    ipc.CodeAgentOffline,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"id":       resp.ID,
		"success":  resp.Success,
		"code":     resp.Code,
		"error":    resp.Error,
		"data":     resp.Data,
		"serverTs": time.Now().Format(time.RFC3339),
	})
}

func (a *uiApp) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, a.pipeName, ipc.Request{Command: ipc.CmdSyncNow})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"success": false,
			"error":   err.Error(),
			"code":    ipc.CodeAgentOffline,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"id":      resp.ID,
		"success": resp.Success,
		"code":    resp.Code,
		"error":   resp.Error,
		"data":    resp.Data,
	})
}

func (a *uiApp) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})

	go func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

const uiHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>LightroomSync UI Test</title>
  <style>
    :root {
      --bg: #f5f8ff;
      --panel: #ffffff;
      --line: #d7e0f7;
      --text: #183153;
      --muted: #58729c;
      --ok: #2e8b57;
      --warn: #c96f00;
      --danger: #c73b3b;
      --btn: #1f5eff;
      --btn2: #0b8a6a;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Segoe UI", Tahoma, sans-serif;
      background: radial-gradient(circle at top left, #eef3ff 0%, var(--bg) 45%, #ecfff9 100%);
      color: var(--text);
      min-height: 100vh;
    }
    .wrap {
      max-width: 980px;
      margin: 22px auto;
      padding: 0 16px;
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 16px;
      box-shadow: 0 10px 30px rgba(24, 49, 83, 0.08);
      padding: 16px;
    }
    h1 {
      margin: 0 0 8px;
      font-size: 22px;
    }
    .sub {
      margin: 0 0 16px;
      color: var(--muted);
      font-size: 14px;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-bottom: 16px;
    }
    button {
      border: 0;
      border-radius: 10px;
      color: #fff;
      padding: 10px 14px;
      font-weight: 600;
      cursor: pointer;
      transition: transform .06s ease, opacity .12s ease;
    }
    button:hover { opacity: 0.92; }
    button:active { transform: translateY(1px); }
    .btn-primary { background: var(--btn); }
    .btn-sync { background: var(--btn2); }
    .btn-danger { background: var(--danger); }
    .status {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 10px;
      margin-bottom: 12px;
    }
    .kv {
      border: 1px solid var(--line);
      border-radius: 10px;
      padding: 10px;
      background: #fcfdff;
    }
    .k { color: var(--muted); font-size: 12px; margin-bottom: 4px; }
    .v { font-size: 14px; font-weight: 600; }
    .v.ok { color: var(--ok); }
    .v.warn { color: var(--warn); }
    .v.danger { color: var(--danger); }
    pre {
      margin: 0;
      border: 1px solid var(--line);
      border-radius: 10px;
      background: #0e1a2b;
      color: #f0f6ff;
      padding: 12px;
      min-height: 240px;
      overflow: auto;
      font-size: 12px;
      line-height: 1.35;
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="panel">
      <h1>Lightroom Sync — Test GUI</h1>
      <p class="sub">Temporary GUI for validating Agent IPC before full Wails UI.</p>
      <div class="actions">
        <button class="btn-primary" id="btnPing">Ping Agent</button>
        <button class="btn-primary" id="btnStatus">Refresh Status</button>
        <button class="btn-sync" id="btnSyncNow">Sync Now</button>
        <button class="btn-danger" id="btnExit">Exit UI</button>
      </div>
      <div class="status">
        <div class="kv"><div class="k">Agent Reachable</div><div class="v" id="agentReachable">Unknown</div></div>
        <div class="kv"><div class="k">Status Text</div><div class="v" id="statusText">-</div></div>
        <div class="kv"><div class="k">Tray Color</div><div class="v" id="trayColor">-</div></div>
        <div class="kv"><div class="k">Lightroom Running</div><div class="v" id="lrRunning">-</div></div>
      </div>
      <pre id="output">Loading...</pre>
    </div>
  </div>
<script>
const $ = (id) => document.getElementById(id);
const output = $("output");

function setClass(el, type) {
  el.classList.remove("ok", "warn", "danger");
  if (type) el.classList.add(type);
}

function write(obj) {
  output.textContent = JSON.stringify(obj, null, 2);
}

function applyStatusEnvelope(env) {
  const ok = !!env.ok;
  $("agentReachable").textContent = ok ? "Yes" : "No";
  setClass($("agentReachable"), ok ? "ok" : "danger");

  const data = env.data || {};
  $("statusText").textContent = data.status_text || (env.error || "-");
  $("trayColor").textContent = data.tray_color || "-";
  $("lrRunning").textContent = (typeof data.lightroom_running === "boolean")
    ? (data.lightroom_running ? "Yes" : "No")
    : "-";
}

async function api(path, method="GET") {
  const res = await fetch(path, { method });
  return await res.json();
}

async function refreshStatus() {
  try {
    const env = await api("/api/status");
    applyStatusEnvelope(env);
    write(env);
  } catch (e) {
    const errObj = { ok: false, error: String(e) };
    applyStatusEnvelope(errObj);
    write(errObj);
  }
}

$("btnPing").addEventListener("click", async () => {
  const env = await api("/api/ping", "POST");
  write(env);
  await refreshStatus();
});

$("btnStatus").addEventListener("click", refreshStatus);

$("btnSyncNow").addEventListener("click", async () => {
  const env = await api("/api/sync-now", "POST");
  write(env);
  setTimeout(refreshStatus, 250);
});

$("btnExit").addEventListener("click", async () => {
  await api("/api/exit", "POST");
  write({ ok: true, message: "UI server shutting down..." });
  setTimeout(() => window.close(), 250);
});

refreshStatus();
setInterval(refreshStatus, 2500);
</script>
</body>
</html>`
