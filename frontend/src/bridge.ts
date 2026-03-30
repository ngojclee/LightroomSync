import type { ActionEnvelope } from "./types";

interface RuntimeBridge {
  executeAction(action: string, payload?: string): Promise<ActionEnvelope> | ActionEnvelope;
}

interface WailsMethodMap {
  ExecuteAction?: (action: string, payload?: string) => Promise<ActionEnvelope> | ActionEnvelope;
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
  if (typeof executeAction === "function") {
    return {
      executeAction(action: string, payload?: string) {
        return executeAction(action, payload);
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
