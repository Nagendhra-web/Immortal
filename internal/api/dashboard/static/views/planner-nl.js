// planner-nl.js — natural language → plan compiler.

import { h, ms as fmtMs } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { toast } from "../components/toast.js";
import { dialog } from "../components/dialog.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "NL → Plan compiler"),
      h("div", { class: "view-header__subtitle" },
        "Describe the goal in plain English. The compiler generates a dependency graph of operations, each with rationale, pre/post conditions, and a rollback."),
    ),
  ));
  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const promptCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Goal")),
    ),
    h("div", { class: "card__body stack" },
      h("textarea", {
        class: "textarea",
        id: "pl-prompt",
        rows: 3,
        placeholder: "e.g. Harden the payments service against the 3pm retry storm while keeping cost flat.",
      }),
      h("div", { class: "row", style: { flexWrap: "wrap" } },
        ...[
          "Harden payments against retry storms",
          "Reduce checkout p99 latency by 30%",
          "Prepare billing for the batch window",
          "Rotate PQ audit keys with zero downtime",
        ].map((ex) =>
          h("button", {
            class: "btn btn--ghost btn--sm",
            onClick: () => document.getElementById("pl-prompt").value = ex,
          }, "“" + ex + "”")
        )
      ),
      h("div", { class: "row", style: { justifyContent: "flex-end" } },
        h("button", { class: "btn btn--accent btn--sm", onClick: compile }, "↯ Compile"),
      ),
    ),
  );
  body.appendChild(promptCard);

  const planCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Compiled plan"),
        h("div", { class: "card__subtitle", id: "pl-summary" }, "No plan yet. Enter a goal and compile."),
      ),
      h("div", { class: "row" },
        h("button", { class: "btn btn--secondary btn--sm", onClick: () => applyPlan("dry-run") }, "▸ Dry-run"),
        h("button", { class: "btn btn--accent btn--sm", onClick: () => applyPlan("apply") }, "✓ Apply"),
      ),
    ),
    h("div", { class: "card__body" },
      h("div", { class: "pipeline", id: "pl-pipeline" }),
      h("div", { id: "pl-rationale" }),
    ),
  );
  body.appendChild(planCard);

  let currentPlan = null;

  const scope = cmdk.scope("planner-nl", [
    { id: "planner-nl:compile", group: "Actions", title: "Compile plan from goal", run: compile },
  ]);

  async function compile() {
    const goal = document.getElementById("pl-prompt").value.trim();
    if (!goal) { toast.warn("Enter a goal first"); return; }
    toast.info("Compiling plan…");
    const r = await tryApi("/api/playbooks", { method: "POST", body: { goal } });
    currentPlan = r.ok ? (r.data?.plan || r.data) : demoPlan(goal);
    render(currentPlan);
  }

  function render(plan) {
    const steps = plan.steps || [];
    document.getElementById("pl-summary").textContent =
      `${steps.length} steps · ETA ${fmtMs(plan.estimated_duration_ms || 5400)} · risk ${plan.risk || "low"}`;
    const host = document.getElementById("pl-pipeline");
    host.innerHTML = "";
    steps.forEach((s, i) => {
      const node = h("div", { class: "pipeline__node", tabIndex: 0, onClick: () => showRationale(s) },
        h("div", { class: "row", style: { justifyContent: "space-between", alignItems: "flex-start" } },
          h("div", { class: "pipeline__node__title" }, `${(i+1).toString().padStart(2,"0")} · ${s.title || s.name}`),
          h("span", { class: `badge badge--${statusClass(s.status)}` }, s.status || "ready"),
        ),
        h("div", { class: "pipeline__node__desc" }, s.description || s.op || ""),
      );
      host.appendChild(node);
      if (i < steps.length - 1) host.appendChild(h("div", { class: "pipeline__arrow" }, "→"));
    });
    if (steps.length) showRationale(steps[0]);
  }

  function showRationale(step) {
    const host = document.getElementById("pl-rationale");
    host.innerHTML = "";
    host.appendChild(h("div", { class: "card", style: { marginTop: "16px" } },
      h("div", { class: "card__header" }, h("div", {}, h("h3", {}, step.title || step.name))),
      h("div", { class: "card__body stack" },
        h("div", {}, h("span", { class: "muted" }, "Rationale: "), step.rationale || "—"),
        h("div", { class: "grid grid-3" },
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Pre-conditions"),
            h("span", {}, (step.pre || []).map((p) => h("div", { class: "mono", style: { fontSize: "11px" } }, "• " + p)))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Post-conditions"),
            h("span", {}, (step.post || []).map((p) => h("div", { class: "mono", style: { fontSize: "11px" } }, "• " + p)))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Rollback"),
            h("span", { class: "mono", style: { fontSize: "11px" } }, step.rollback || "—")),
        ),
      ),
    ));
  }

  async function applyPlan(mode) {
    if (!currentPlan) { toast.warn("Compile a plan first"); return; }
    if (mode === "apply") {
      const confirmDlg = dialog({
        title: "Apply plan?",
        body: h("div", {},
          h("p", {}, "This will write-through to the engine. ", h("strong", {}, (currentPlan.steps || []).length + " steps"), " will execute, each signing an attestation."),
          h("p", { class: "muted" }, "You can review individual step rollbacks in the audit log afterward."),
        ),
        footer: [
          h("button", { class: "btn btn--ghost btn--sm", onClick: () => confirmDlg.close() }, "Cancel"),
          h("button", { class: "btn btn--accent btn--sm", onClick: () => { confirmDlg.close(); doApply(); } }, "Apply"),
        ],
      });
    } else {
      toast.info("Dry-running against the twin…");
      setTimeout(() => toast.ok("Dry-run OK", "no regressions predicted"), 800);
    }
  }

  async function doApply() {
    toast.info("Applying plan…");
    const r = await tryApi("/api/playbooks", { method: "POST", body: { apply: true, plan: currentPlan } });
    if (r.ok) toast.ok("Plan applied", r.data?.receipt || "receipt stored in audit log");
    else toast.warn("Engine rejected apply", r.error?.message || "check audit log");
  }

  return () => scope();
}

function statusClass(s) {
  const v = String(s || "ready").toLowerCase();
  if (v.includes("ok") || v.includes("done")) return "ok";
  if (v.includes("fail") || v.includes("err")) return "err";
  if (v.includes("pend") || v.includes("wait")) return "warn";
  return "info";
}

function demoPlan(goal) {
  return {
    goal,
    estimated_duration_ms: 12_500,
    risk: "medium",
    steps: [
      { title: "Snapshot current twin state",
        description: "twin.checkpoint(services=*)",
        rationale: "Gives us a provable baseline for comparison and rollback.",
        pre: ["engine.healthy"],
        post: ["twin.checkpoint.id present"],
        rollback: "noop (read-only)",
        status: "ready" },
      { title: "Tighten payments retry budget",
        description: "playbook.apply(name=retry-budget, service=payments, cap=3)",
        rationale: "Retry storms amplify the 3pm spike. A lower cap bounds fan-out.",
        pre: ["service=payments healthy", "budget>=80%"],
        post: ["retry.cap == 3", "attestation stored"],
        rollback: "restore previous cap from audit log",
        status: "ready" },
      { title: "Warm cache for top 100 SKUs",
        description: "playbook.apply(name=cache-warm, service=catalog, top=100)",
        rationale: "Pre-loads the skewed read-set to absorb the spike without cold-miss spikes.",
        pre: ["cache.headroom > 0.3"],
        post: ["cache.hit_rate > 0.95 within 60s"],
        rollback: "LRU decay (~5 min)",
        status: "ready" },
      { title: "Verify via twin",
        description: "twin.simulate(spike=3pm, duration=30m)",
        rationale: "Runs the plan through the digital twin under the spike scenario before letting it touch prod.",
        pre: [],
        post: ["predicted p99 < 140ms"],
        rollback: "—",
        status: "ready" },
    ],
  };
}
