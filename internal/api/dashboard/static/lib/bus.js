// bus.js — tiny pub/sub. Carries global state across views.
// API: bus.on(event, fn), bus.off(event, fn), bus.emit(event, payload), bus.get(event)

const listeners = new Map();
const lastPayload = new Map();

export const bus = {
  on(event, fn) {
    if (!listeners.has(event)) listeners.set(event, new Set());
    listeners.get(event).add(fn);
    if (lastPayload.has(event)) fn(lastPayload.get(event));
    return () => this.off(event, fn);
  },
  off(event, fn) {
    const set = listeners.get(event);
    if (set) set.delete(fn);
  },
  emit(event, payload) {
    lastPayload.set(event, payload);
    const set = listeners.get(event);
    if (!set) return;
    for (const fn of set) {
      try { fn(payload); } catch (e) { console.error("bus listener failed", event, e); }
    }
  },
  get(event) { return lastPayload.get(event); },
};

// Persisted app state (time range, env, paused).
const STORAGE_KEY = "immortal:ui:state";
const defaultState = {
  range: { preset: "1h", t0: null, t1: null },
  env: "local",
  paused: false,
};

export function loadState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...defaultState };
    return { ...defaultState, ...JSON.parse(raw) };
  } catch {
    return { ...defaultState };
  }
}

export function saveState(state) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); } catch {}
}
