function envValue(key: string): string {
  const value = (globalThis as Record<string, unknown>)[key];
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  return "unknown";
}

export function renderApp(): string {
  const pipeName = envValue("LIGHTROOMSYNC_PIPE");
  const version = envValue("LIGHTROOMSYNC_UI_VERSION");

  return `
    <main class="shell">
      <header class="hero">
        <h1>Lightroom Sync</h1>
        <p class="subtitle">Wave 1 Wails Bootstrap (Placeholder UI)</p>
      </header>

      <section class="panel">
        <h2>Runtime Context</h2>
        <dl>
          <dt>UI Version</dt>
          <dd>${version}</dd>
          <dt>Pipe</dt>
          <dd>${pipeName}</dd>
          <dt>Status</dt>
          <dd>Wails shell scaffold is ready. Functional tabs start at Wave 3.</dd>
        </dl>
      </section>
    </main>
  `;
}

