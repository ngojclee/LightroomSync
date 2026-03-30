import type { ActionEnvelope } from "./types";

interface RuntimeBridge {
  executeAction(action: string, payload?: string): Promise<ActionEnvelope> | ActionEnvelope;
  selectDirectory?(title: string): Promise<string> | string;
  selectFile?(title: string, filters: string): Promise<string> | string;
}

interface WailsMethodMap {
  ExecuteAction?: (action: string, payload?: string) => Promise<ActionEnvelope> | ActionEnvelope;
  SelectDirectory?: (title: string) => Promise<string> | string;
  SelectFile?: (title: string, filters: string) => Promise<string> | string;
  ExitApplication?: () => Promise<ActionEnvelope> | ActionEnvelope;
  QuitUIOnly?: () => Promise<void> | void;
  IsAgentAlive?: () => Promise<boolean> | boolean;
  LaunchAgent?: () => Promise<Record<string, string>> | Record<string, string>;
  SetMinimizeToTray?: (enabled: boolean) => Promise<void> | void;
  HideToTray?: () => Promise<void> | void;
  MinimiseWindow?: () => Promise<void> | void;
  ShowWindow?: () => Promise<void> | void;
}

interface WailsMainMap {
  WailsApp?: WailsMethodMap;
}

interface WailsGoMap {
  main?: WailsMainMap;
}

declare global {
  interface Window {
    LightroomSyncBridge?: RuntimeBridge;
    go?: WailsGoMap;
  }
}

function nowIso(): string {
  return new Date().toISOString();
}

function offlineEnvelope(reason: string): ActionEnvelope {
  return {
    ok: false,
    success: false,
    code: "agent_offline",
    error: reason,
    server_ts: nowIso()
  };
}

function normalizeEnvelope(raw: unknown): ActionEnvelope {
  const obj = (raw ?? {}) as Record<string, unknown>;
  return {
    ok: Boolean(obj.ok),
    id: typeof obj.id === "string" ? obj.id : undefined,
    success: Boolean(obj.success),
    code: typeof obj.code === "string" ? obj.code : undefined,
    error: typeof obj.error === "string" ? obj.error : undefined,
    data: obj.data,
    server_ts: typeof obj.server_ts === "string" ? obj.server_ts : nowIso()
  };
}

function resolveBridge(): RuntimeBridge | null {
  if (window.LightroomSyncBridge && typeof window.LightroomSyncBridge.executeAction === "function") {
    return window.LightroomSyncBridge;
  }

  const executeAction = window.go?.main?.WailsApp?.ExecuteAction;
  const selectDirectoryAttr = window.go?.main?.WailsApp?.SelectDirectory;
  const selectFileAttr = window.go?.main?.WailsApp?.SelectFile;

  if (typeof executeAction === "function") {
    return {
      executeAction(action: string, payload?: string) {
        return executeAction(action, payload);
      },
      selectDirectory(title: string) {
        return typeof selectDirectoryAttr === "function" 
            ? selectDirectoryAttr(title) 
            : "";
      },
      selectFile(title: string, filters: string) {
        return typeof selectFileAttr === "function" 
            ? selectFileAttr(title, filters) 
            : "";
      }
    };
  }

  return null;
}

export async function executeAction(action: string, payload = ""): Promise<ActionEnvelope> {
  const bridge = resolveBridge();
  if (!bridge) {
    return offlineEnvelope("Frontend bridge is unavailable (Wails binding not connected).");
  }

  try {
    const result = await bridge.executeAction(action, payload);
    return normalizeEnvelope(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown bridge error";
    return offlineEnvelope(message);
  }
}

export async function selectDirectory(title: string): Promise<string> {
  const bridge = resolveBridge();
  if (!bridge || typeof bridge.selectDirectory !== "function") {
    console.warn("Wails Native Dialog not available for SelectDirectory; fallback needed or unsupported environment");
    return "";
  }
  
  try {
    const dir = await bridge.selectDirectory(title);
    return typeof dir === "string" ? dir : "";
  } catch (err) {
    console.error("Select directory error:", err);
    return "";
  }
}

export async function selectFile(title: string, filters = ""): Promise<string> {
  const bridge = resolveBridge();
  if (!bridge || typeof bridge.selectFile !== "function") {
    console.warn("Wails Native Dialog not available for SelectFile");
    return "";
  }
  
  try {
    const file = await bridge.selectFile(title, filters);
    return typeof file === "string" ? file : "";
  } catch (err) {
    console.error("Select file error:", err);
    return "";
  }
}

export async function exitApplication(): Promise<ActionEnvelope> {
  const fn = window.go?.main?.WailsApp?.ExitApplication;
  if (typeof fn === "function") {
    try {
      const result = await fn();
      return normalizeEnvelope(result);
    } catch {
      return offlineEnvelope("Exit failed");
    }
  }
  return offlineEnvelope("ExitApplication binding unavailable");
}

export async function quitUIOnly(): Promise<void> {
  const fn = window.go?.main?.WailsApp?.QuitUIOnly;
  if (typeof fn === "function") {
    await fn();
  }
}

export async function isAgentAlive(): Promise<boolean> {
  const fn = window.go?.main?.WailsApp?.IsAgentAlive;
  if (typeof fn === "function") {
    try {
      return await fn();
    } catch {
      return false;
    }
  }
  // Fallback: try a ping via IPC
  const result = await executeAction("ping");
  return result.ok;
}

export async function launchAgent(): Promise<Record<string, string>> {
  const fn = window.go?.main?.WailsApp?.LaunchAgent;
  if (typeof fn === "function") {
    try {
      return await fn();
    } catch (err) {
      return { ok: "false", error: String(err) };
    }
  }
  return { ok: "false", error: "LaunchAgent binding unavailable" };
}

export function syncMinimizeToTray(enabled: boolean): void {
  const fn = window.go?.main?.WailsApp?.SetMinimizeToTray;
  if (typeof fn === "function") {
    fn(enabled);
  }
}

export function hideToTray(): void {
  const fn = window.go?.main?.WailsApp?.HideToTray;
  if (typeof fn === "function") {
    fn();
  }
}

export function minimiseWindow(): void {
  const fn = window.go?.main?.WailsApp?.MinimiseWindow;
  if (typeof fn === "function") {
    fn();
  }
}

export async function discoverPresets(): Promise<string[]> {
  const result = await executeAction("discover-presets");
  if (result.ok && result.success && Array.isArray(result.data)) {
    return result.data;
  }
  return [];
}
