// evolve.js — architecture advisor view.
// Scored structural suggestions (AddCache, SplitService, AddCircuitBreaker, ...)
// with evidence, predicted impact, and effort classification.

import { h, escapeHtml, num } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { toast } from "../components/toast.js";
import { cmdk } from "../lib/cmdk-registry.js";

const RANK_COLOR = {
  critical: "err",
  high:     "warn",
  medium:   "info",
  low:      "ok",
};

const EFFORT_COLOR = {
  small:  "ok",
  medium: "warn",
  large:  "err",
};

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Architecture advisor"),
      h("div", { class: "view-header__subtitle" },
        "Scored suggestions for structural changes. Each carries evidence, twin-simulated impact, and an effort estimate so you can triage."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const meta = h("div", { class: "row muted", id: "ev-meta" });
  body.appendChild(meta);

  const grid = h("div", { class: "grid grid-auto-md", id: "ev-grid" });
  body.appendChild(grid);

  refresh();
  const scope = cmdk.scope("evolve", [
    { id: "evolve:refresh", group: "Actions", title: "Refresh architecture suggestions", run: refresh },
  ]);

  async function refresh() {
    const res = await tryApi("/api/v6/evolve/suggest");
    const meta = document.getElementById("ev-meta");
    const grid = document.getElementById("ev-grid");
    meta.innerHTML = "";
    grid.innerHTML = "";
    if (!res.ok) {
      grid.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "Advisor unavailable"),
        h("div", {}, res.error?.message || "Engine is not running with evolve enabled."),
      ));
      return;
    }
    const suggestions = res.data?.suggestions || [];
    meta.appendChild(h("span", {}, `${suggestions.length} suggestions`));
    if (res.data?.generated_at) {
      meta.appendChild(h("span", { style: { marginLeft: "16px" } }, "generated " + res.data.generated_at));
    }
    if (!suggestions.length) {
      grid.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "No structural changes needed"),
        h("div", {}, "Your signals look clean. Check back after the next incident."),
      ));
      return;
    }
    suggestions.forEach((s) => grid.appendChild(card(s)));
  }

  function card(s) {
    const rank = s.rank || s.Rank || "low";
    const effort = s.Effort || "medium";
    const evidence = s.Evidence || [];
    const impact = s.Impact || "";
    const title = `${s.Kind || "suggestion"} on ${s.Service || "system"}`;

    return h("section", { class: "card" },
      h("header", { class: "card__header" },
        h("div", { style: { flex: 1 } },
          h("div", { class: "row" },
            h("span", { class: `badge badge--${RANK_COLOR[rank] || "info"}` }, rank),
            h("span", { class: `badge badge--${EFFORT_COLOR[effort] || "info"}`, style: { marginLeft: "6px" } }, effort + " effort"),
            h("span", { class: "muted", style: { marginLeft: "auto", fontFamily: "var(--font-mono)", fontSize: "11px" } }, "score " + num(s.Score ?? 0, 2)),
          ),
          h("div", { class: "card__title", style: { marginTop: "6px" } }, title),
        ),
      ),
      h("div", { class: "card__body stack" },
        h("div", {}, s.Rationale || ""),
        evidence.length ? h("div", {},
          h("div", { class: "muted", style: { fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.08em", marginBottom: "4px" } }, "Evidence"),
          h("ul", { style: { margin: 0, paddingLeft: "18px" } }, ...evidence.map((e) => h("li", { class: "mono", style: { fontSize: "12px" } }, e))),
        ) : null,
        impact ? h("div", { class: "muted", style: { padding: "8px 10px", background: "var(--bg-elevated)", borderRadius: "var(--r-sm)", fontSize: "12px" } }, impact) : null,
        h("div", { class: "row" },
          h("button", {
            class: "btn btn--secondary btn--sm",
            onClick: () => toast.info("Twin simulation", "Not yet wired to real twin in this demo pass."),
          }, "Simulate in twin"),
          h("button", {
            class: "btn btn--ghost btn--sm",
            onClick: () => copyImpact(s),
          }, "Copy impact"),
        ),
      ),
    );
  }

  function copyImpact(s) {
    const text = `${s.Kind} on ${s.Service} (score ${num(s.Score ?? 0, 2)}): ${s.Rationale}\nImpact: ${s.Impact}`;
    navigator.clipboard?.writeText(text).then(
      () => toast.ok("Copied"),
      () => toast.err("Copy failed"),
    );
  }

  return () => scope();
}
