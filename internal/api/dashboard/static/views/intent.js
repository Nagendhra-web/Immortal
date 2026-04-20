// intent.js — intent-based healing view.
// Shows registered intents + live goal status; lets operators apply
// preset contracts and see the ranked suggestions compiled from violations.

import { h, escapeHtml, num, pct, ms as fmtMs } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { toast } from "../components/toast.js";
import { table } from "../components/table.js";
import { cmdk } from "../lib/cmdk-registry.js";

const PRESETS = [
  {
    name: "protect-checkout",
    title: "Protect checkout",
    description: "Latency < 200 ms, errors < 0.5% on checkout and payments.",
    body: {
      name: "protect-checkout",
      goals: [
        { kind: 5, service: "checkout", priority: 10 },       // ProtectService
        { kind: 0, service: "checkout", target: 200, priority: 10 }, // LatencyUnder
        { kind: 1, service: "checkout", target: 0.005, priority: 10 }, // ErrorRateUnder
        { kind: 5, service: "payments", priority: 10 },
        { kind: 0, service: "payments", target: 200, priority: 10 },
        { kind: 1, service: "payments", target: 0.005, priority: 10 },
      ],
    },
  },
  {
    name: "never-drop-jobs",
    title: "Never drop jobs",
    description: "Queue must retain every accepted unit of work.",
    body: {
      name: "never-drop-jobs",
      goals: [{ kind: 3, service: "queue", target: 0, priority: 10 }], // JobsNoDrop
    },
  },
  {
    name: "available-under-degradation",
    title: "Available under degradation",
    description: "Availability > 99%, error rate < 5% even during incidents.",
    body: {
      name: "available-under-degradation",
      goals: [
        { kind: 2, service: "api", target: 0.99, priority: 7 }, // AvailabilityOver
        { kind: 1, service: "api", target: 0.05, priority: 7 },
      ],
    },
  },
  {
    name: "cost-ceiling",
    title: "Cost ceiling",
    description: "Cap engine spend at $12/hour before degrading non-critical.",
    body: {
      name: "cost-ceiling",
      goals: [{ kind: 6, service: "", target: 12.0, priority: 4 }], // CostCap
    },
  },
];

const STATUS_LABEL = { true: "met", false: "violated" };

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Intent-based healing"),
      h("div", { class: "view-header__subtitle" },
        "Declare what must stay true. The engine chooses the cheapest set of actions that keeps every goal met, and degrades non-critical paths first."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  // ── Preset contracts ────────────────────────────────────────────────
  body.appendChild(h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Contracts you can register")),
    ),
    h("div", { class: "card__body grid grid-auto-sm" },
      ...PRESETS.map((p) => h("div", { class: "kpi", style: { gap: "var(--s-2)" } },
        h("span", { class: "kpi__label" }, p.title),
        h("div", { class: "muted", style: { fontSize: "12px", marginBottom: "8px" } }, p.description),
        h("button", {
          class: "btn btn--accent btn--sm",
          onClick: () => addPreset(p),
        }, "Register"),
      )),
    ),
  ));

  // ── Registered contracts + statuses ─────────────────────────────────
  const registeredCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Registered contracts")),
    ),
    h("div", { class: "card__body", id: "in-contracts" }),
  );
  body.appendChild(registeredCard);

  // ── Ranked suggestions ──────────────────────────────────────────────
  const suggestCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Ranked suggestions"),
        h("div", { class: "card__subtitle" }, "Highest-priority violated goals first."),
      ),
    ),
    h("div", { class: "card__body", id: "in-suggestions" }),
  );
  body.appendChild(suggestCard);

  refresh();
  const scope = cmdk.scope("intent", [
    { id: "intent:refresh", group: "Actions", title: "Refresh intent view", run: refresh },
    ...PRESETS.map((p) => ({
      id: `intent:preset:${p.name}`, group: "Intent presets",
      title: `Register contract: ${p.title}`, run: () => addPreset(p),
    })),
  ]);

  async function addPreset(p) {
    toast.info("Registering contract…", p.title);
    const r = await tryApi("/api/v6/intent", { method: "POST", body: p.body });
    if (r.ok) { toast.ok("Contract registered", p.title); refresh(); }
    else toast.err("Failed to register", r.error?.message);
  }

  async function removeOne(name) {
    const r = await tryApi(`/api/v6/intent/${encodeURIComponent(name)}`, { method: "DELETE" });
    if (r.ok) { toast.ok("Contract removed", name); refresh(); }
    else toast.err("Failed to remove", r.error?.message);
  }

  async function refresh() {
    const [list, suggest] = await Promise.all([
      tryApi("/api/v6/intent"),
      tryApi("/api/v6/intent/suggest"),
    ]);
    renderContracts(list);
    renderSuggestions(suggest);
  }

  function renderContracts(res) {
    const host = document.getElementById("in-contracts");
    host.innerHTML = "";
    if (!res.ok) {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "Intent evaluator not configured"),
        h("div", {}, "Start the engine with the intent feature enabled to use this view."),
      ));
      return;
    }
    const rows = res.data || [];
    if (!rows.length) {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "No contracts registered yet"),
        h("div", {}, "Click a preset above to declare your first contract."),
      ));
      return;
    }
    const wrap = h("div", { class: "stack" });
    rows.forEach((row) => {
      const header = h("div", { class: "row", style: { justifyContent: "space-between", alignItems: "flex-start" } },
        h("div", {},
          h("div", { class: "strong" }, row.intent.Name),
          h("div", { class: "muted", style: { fontSize: "12px" } }, row.summary || "(custom contract)"),
        ),
        h("button", {
          class: "btn btn--ghost btn--sm",
          onClick: () => removeOne(row.intent.Name),
          "data-tooltip": "Remove contract",
        }, "✕"),
      );
      const statuses = row.statuses || [];
      const tbl = table({
        columns: [
          { label: "Goal", key: "Goal", render: (s) => goalLabel(s.Goal) },
          { label: "Target", key: "Target", align: "right", render: (s) => num(s.Target, 2) },
          { label: "Current", key: "Current", align: "right", render: (s) => s.Current != null ? num(s.Current, 2) : "—" },
          { label: "Status", key: "Met", render: (s) => {
            const cls = !s.Met ? "err" : s.AtRisk ? "warn" : "ok";
            const label = !s.Met ? "violated" : s.AtRisk ? "at risk" : "met";
            return h("span", { class: `badge badge--${cls}` }, label);
          }},
        ],
        rows: statuses,
        emptyText: "No goals in this contract.",
      });
      wrap.appendChild(h("section", { class: "card", style: { background: "var(--bg-elevated)", padding: "12px 14px" } }, header, tbl.el));
    });
    host.appendChild(wrap);
  }

  function renderSuggestions(res) {
    const host = document.getElementById("in-suggestions");
    host.innerHTML = "";
    if (!res.ok) {
      host.appendChild(h("div", { class: "empty" }, h("div", { class: "empty__title" }, "No suggestions available")));
      return;
    }
    const rows = res.data || [];
    if (!rows.length) {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "All goals met"),
        h("div", {}, "No corrective actions needed right now."),
      ));
      return;
    }
    const wrap = h("div", { class: "stack" });
    rows.forEach((s) => {
      wrap.appendChild(h("div", { class: "card", style: { padding: "12px 14px" } },
        h("div", { class: "row", style: { justifyContent: "space-between" } },
          h("span", { class: "mono strong" }, s.Action),
          h("span", { class: "badge badge--violet" }, "priority " + (s.Priority ?? 0)),
        ),
        h("div", { class: "muted", style: { marginTop: "6px", fontSize: "13px" } }, s.Rationale),
        s.Impact && Object.keys(s.Impact).length ? h("div", { class: "mono muted", style: { marginTop: "6px", fontSize: "11px" } },
          Object.entries(s.Impact).map(([k, v]) => `${k}=${num(v, 2)}`).join(" · "),
        ) : null,
      ));
    });
    host.appendChild(wrap);
  }

  return () => scope();
}

function goalLabel(g) {
  const kinds = ["latency<", "error<", "avail>", "jobs=0", "protect", "cost<", "sat<"];
  const kind = kinds[g.Kind] ?? "goal";
  const svc = g.Service ? `${g.Service}:` : "";
  return svc + kind + " " + (g.Target ?? "");
}
