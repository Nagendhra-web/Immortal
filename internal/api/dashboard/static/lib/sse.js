// sse.js — EventSource wrapper with auto-reconnect + paused state.
// Usage: const stream = sse("/api/events", { onMessage, onError }); stream.close();

import { bus } from "./bus.js";

export function sse(path, { onMessage, onEvent, onError, events = [] } = {}) {
  let es = null;
  let closed = false;
  let paused = bus.get("app:paused") || false;
  const unsubPause = bus.on("app:paused", (p) => {
    paused = p;
    if (p) { try { es && es.close(); } catch {} es = null; }
    else if (!es && !closed) connect();
  });

  function connect() {
    if (paused || closed) return;
    try {
      es = new EventSource(path);
      es.onmessage = (e) => { if (!paused) onMessage && onMessage(e.data, e); };
      if (events.length && onEvent) {
        for (const name of events) {
          es.addEventListener(name, (e) => { if (!paused) onEvent(name, e.data, e); });
        }
      }
      es.onerror = (e) => {
        onError && onError(e);
        // Browsers auto-reconnect; if the stream was canceled we close.
      };
    } catch (err) {
      onError && onError(err);
    }
  }
  connect();
  return {
    close() {
      closed = true;
      unsubPause();
      if (es) { try { es.close(); } catch {} es = null; }
    },
  };
}
