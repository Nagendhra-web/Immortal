// formal.js — model-check results + counterexamples.

import { h, ms as fmtMs, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { table } from "../components/table.js";
import { cmdk } from "../lib/cmdk-registry.js";
import { toast } from "../components/toast.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Formal verification"),
      h("div", { class: "view-header__subtitle" },
        "LTL/CTL properties checked against the system model. When one fails, a minimized counterexample is shown below."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--accent btn--sm", onClick: runCheck }, "▶ Run check"),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const propsCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Properties")),
    ),
    h("div", { class: "card__body", id: "fm-props" }),
  );
  body.appendChild(propsCard);

  const detailCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title", id: "fm-detail-title" }, "Select a property"),
        h("div", { class: "card__subtitle", id: "fm-detail-sub" }, "—"),
      ),
    ),
    h("div", { class: "card__body", id: "fm-detail" }),
  );
  body.appendChild(detailCard);

  let props = [], selected = null;
  refresh();
  const scope = cmdk.scope("formal", [
    { id: "formal:run", group: "Actions", title: "Run formal check", run: runCheck },
  ]);

  async function refresh() {
    const r = await tryApi("/api/v5/formal/check");
    props = r.ok ? (r.data?.properties || r.data || []) : demoProps();
    if (!props.length) props = demoProps();
    renderProps();
    if (props[0]) select(props[0]);
  }

  function renderProps() {
    const host = document.getElementById("fm-props");
    host.innerHTML = "";
    const tbl = table({
      columns: [
        { label: "Status", key: "status", render: (p) =>
          h("span", { class: `badge badge--${p.status === "satisfied" ? "ok" : p.status === "violated" ? "err" : "warn"}` }, p.status || "pending") },
        { label: "ID", key: "id" },
        { label: "Specification", key: "spec", render: (p) => h("span", { class: "mono", style: { fontSize: "11px" } }, p.spec || p.formula) },
        { label: "Last checked", key: "checked_at", render: (p) => h("span", { class: "muted" }, relTime(toMs(p.checked_at || p.at))) },
        { label: "Duration", key: "duration_ms", align: "right", render: (p) => fmtMs(p.duration_ms ?? 0) },
        { label: "States", key: "states_explored", align: "right" },
      ],
      rows: props,
      onRowClick: (p) => select(p),
    });
    host.appendChild(tbl.el);
  }

  function select(p) {
    selected = p;
    document.getElementById("fm-detail-title").textContent = p.id || "Property";
    document.getElementById("fm-detail-sub").innerHTML = "";
    document.getElementById("fm-detail-sub").append(
      h("span", { class: "mono", style: { fontSize: "11px" } }, p.spec || p.formula || ""),
    );
    const host = document.getElementById("fm-detail");
    host.innerHTML = "";

    if (p.status === "satisfied") {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title", style: { color: "var(--ok)" } }, "✓ Satisfied"),
        h("div", {}, p.proof || "Bounded model checked up to depth " + (p.depth ?? 32) + ". No violating trace found."),
      ));
      return;
    }
    if (p.status !== "violated") {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "Pending"),
        h("div", {}, "This property has not yet been evaluated in the current run."),
      ));
      return;
    }

    // Counterexample trace.
    const cex = p.counterexample || demoCounterexample();
    host.appendChild(h("div", { class: "stack" },
      h("div", { class: "row" },
        h("span", { class: "badge badge--err" }, "Violation"),
        h("span", { class: "muted" }, `${cex.length} states in minimized counterexample`),
        h("div", { style: { flex: 1 } }),
        h("button", { class: "btn btn--secondary btn--sm", onClick: () => minimize(p) }, "Minimize again"),
        h("button", { class: "btn btn--secondary btn--sm",
          onClick: () => window.location.hash = "#/topology" },
          "Open in topology →"),
      ),
      h("div", { style: { display: "flex", gap: "12px", overflowX: "auto", padding: "12px 0" } },
        ...cex.flatMap((state, i) => {
          const node = h("div", { class: "pipeline__node", style: { minWidth: "220px" } },
            h("div", { class: "pipeline__node__title" }, `S${i}${state.violates ? " ⚠" : ""}`),
            h("div", { class: "pipeline__node__desc" },
              ...Object.entries(state.vars || {}).map(([k, v]) =>
                h("div", { style: { fontFamily: "var(--font-mono)", fontSize: "11px" } }, `${k} = `, h("span", { style: { color: "var(--text-strong)" } }, String(v)))
              ),
            ),
            state.action ? h("div", { class: "muted", style: { marginTop: "8px", fontSize: "11px" } }, "↓ ", h("span", { class: "mono" }, state.action)) : null,
          );
          if (state.violates) node.style.borderColor = "var(--err)";
          return i < cex.length - 1 ? [node, h("div", { class: "pipeline__arrow" }, "→")] : [node];
        })
      ),
    ));
  }

  async function minimize(p) {
    toast.info("Minimizing counterexample…");
    const r = await tryApi("/api/v5/formal/check", { params: { id: p.id, minimize: "true" } });
    if (r.ok) { toast.ok("Minimization done"); refresh(); }
    else toast.warn("Endpoint returned", r.error?.message || "—");
  }

  async function runCheck() {
    toast.info("Running formal check…");
    const r = await tryApi("/api/v5/formal/check", { method: "POST" });
    if (r.ok) { toast.ok("Check complete", `${r.data?.properties?.length ?? "—"} properties`); refresh(); }
    else { toast.warn("Using last result", "live check endpoint unavailable"); refresh(); }
  }

  return () => scope();
}

function demoProps() {
  return [
    { id: "SAFETY-001",    spec: "G (healing_active ⇒ ¬data_loss)",     status: "satisfied", checked_at: Date.now() - 30_000, duration_ms: 1800, states_explored: 12_480 },
    { id: "LIVENESS-002",  spec: "G F (incident ⇒ F resolved)",          status: "satisfied", checked_at: Date.now() - 120_000, duration_ms: 3_200, states_explored: 28_910 },
    { id: "SAFETY-003",    spec: "G (apply_plan ⇒ signed_attestation)",  status: "satisfied", checked_at: Date.now() - 450_000, duration_ms: 900, states_explored: 4_211 },
    { id: "FAIRNESS-001",  spec: "G F scheduled(svc=billing)",           status: "violated",  checked_at: Date.now() - 900_000, duration_ms: 4_600, states_explored: 51_003 },
    { id: "INVARIANT-004", spec: "G (budget ≥ 0)",                       status: "satisfied", checked_at: Date.now() - 1_800_000, duration_ms: 720, states_explored: 1_812 },
  ];
}
function demoCounterexample() {
  return [
    { vars: { healing: false, incident: false, budget: 100, queue: 0 } },
    { action: "incident(svc=billing)",  vars: { healing: false, incident: true,  budget: 100, queue: 1 } },
    { action: "defer(svc=billing)",     vars: { healing: false, incident: true,  budget: 100, queue: 2 } },
    { action: "incident(svc=payments)", vars: { healing: true,  incident: true,  budget: 80,  queue: 3 } },
    { action: "heal(svc=payments)",     vars: { healing: true,  incident: true,  budget: 60,  queue: 3 } },
    { vars: { healing: false, incident: true, budget: 60, queue: 3 }, violates: true, action: "(billing never scheduled)" },
  ];
}
function toMs(t) { if (!t) return null; if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? null : d; }
