// cmdk-registry.js — command palette registry. Views register/deregister on mount/unmount.

const STORAGE_RECENT = "immortal:ui:cmdk:recent";

const commands = new Map();   // id -> command
const perView  = new Map();   // viewId -> Set<id>

export const cmdk = {
  register(cmd) {
    commands.set(cmd.id, cmd);
  },
  registerMany(list) { for (const c of list) this.register(c); },

  deregister(id) { commands.delete(id); },

  scope(viewId, list) {
    if (!perView.has(viewId)) perView.set(viewId, new Set());
    const set = perView.get(viewId);
    for (const c of list) {
      this.register(c);
      set.add(c.id);
    }
    return () => this.clearScope(viewId);
  },
  clearScope(viewId) {
    const set = perView.get(viewId);
    if (!set) return;
    for (const id of set) commands.delete(id);
    perView.delete(viewId);
  },

  all() { return Array.from(commands.values()); },

  search(query) {
    const recents = loadRecents();
    const list = Array.from(commands.values());
    if (!query) {
      return list
        .map((c) => ({ c, score: recents[c.id] ? 1000 + recents[c.id] : 0 }))
        .sort((a, b) => b.score - a.score)
        .map((x) => x.c);
    }
    const q = query.toLowerCase();
    const scored = [];
    for (const c of list) {
      const score = scoreMatch(c, q) + (recents[c.id] ? 2 : 0);
      if (score > 0) scored.push({ c, score });
    }
    scored.sort((a, b) => b.score - a.score);
    return scored.map((x) => x.c);
  },

  run(cmd) {
    markRecent(cmd.id);
    try { cmd.run && cmd.run(); } catch (e) { console.error("cmdk run failed", e); }
  },
};

function scoreMatch(cmd, q) {
  const hay = [
    cmd.title,
    cmd.group || "",
    ...(cmd.keywords || []),
  ].join(" ").toLowerCase();
  if (hay.includes(q)) return 100 - (hay.indexOf(q));
  // subsequence
  let i = 0;
  for (const ch of hay) { if (ch === q[i]) i++; if (i >= q.length) break; }
  if (i >= q.length) return 20;
  return 0;
}

function loadRecents() {
  try { return JSON.parse(localStorage.getItem(STORAGE_RECENT) || "{}"); }
  catch { return {}; }
}
function markRecent(id) {
  const r = loadRecents();
  r[id] = Date.now();
  try { localStorage.setItem(STORAGE_RECENT, JSON.stringify(r)); } catch {}
}
