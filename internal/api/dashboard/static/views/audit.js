// audit.js — Merkle-anchored audit log + verification.

import { h, num, dateShort, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { table } from "../components/table.js";
import { sheet } from "../components/sheet.js";
import { toast } from "../components/toast.js";
import { parseFilter } from "../lib/filter-dsl.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Audit log"),
      h("div", { class: "view-header__subtitle" }, "Append-only, Merkle-anchored, post-quantum-signed audit trail. Every decision the engine makes leaves a receipt."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--accent btn--sm", onClick: verifyAll }, "✓ Verify chain"),
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body" });
  root.appendChild(body);

  const summary = h("div", { class: "grid grid-auto-sm" });
  body.appendChild(summary);
  const cards = {
    entries:   kpi("Entries", "—"),
    root:      kpi("Merkle root", "—"),
    last:      kpi("Last verified", "—"),
    attests:   kpi("Attestations", "—"),
  };
  for (const k of Object.values(cards)) summary.appendChild(k.el);

  const tableCard = h("section", { class: "card", style: { marginTop: "var(--s-4)" } },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Entries")),
      h("div", { class: "row" },
        h("input", {
          class: "input input--mono",
          style: { width: "340px" },
          placeholder: "kind=heal or service~payments",
          dataset: { role: "view-filter" },
          onInput: (e) => applyFilter(e.target.value),
        }),
      ),
    ),
    h("div", { class: "card__body", id: "audit-table" }),
  );
  body.appendChild(tableCard);

  let rows = [], predicate = () => true, tbl;
  refresh();
  const scope = cmdk.scope("audit", [
    { id: "audit:verify", group: "Actions", title: "Verify Merkle chain", run: verifyAll },
  ]);

  async function refresh() {
    const [entries, root, verify] = await Promise.all([
      tryApi("/api/v4/audit/entries", { params: { limit: 500 } }),
      tryApi("/api/v4/audit/merkle-root"),
      tryApi("/api/audit"),
    ]);
    rows = entries.ok ? (entries.data?.entries || entries.data || []) : (verify.ok ? (verify.data?.entries || []) : []);
    cards.entries.setValue(num(rows.length, 0));
    const merkle = root.ok ? (root.data?.root || root.data) : null;
    cards.root.setValue(merkle ? shortHash(String(merkle)) : "—");
    cards.last.setValue(verify.ok ? relTime(Date.now()) : "—");
    const attCount = rows.filter((r) => r.attestation || r.signature).length;
    cards.attests.setValue(String(attCount));
    render();
  }

  function applyFilter(q) {
    try { predicate = parseFilter(q); render(); }
    catch { predicate = () => true; render(); }
  }

  function render() {
    const host = document.getElementById("audit-table");
    host.innerHTML = "";
    const data = rows.filter(predicate);
    tbl = table({
      columns: [
        { label: "#",       key: "index", align: "right" },
        { label: "Kind",    key: "kind",   render: (r) => h("span", { class: `badge badge--${kindClass(r.kind)}` }, r.kind || "info") },
        { label: "Service", key: "service" },
        { label: "Summary", key: "summary", value: (r) => r.summary || r.action || r.title },
        { label: "Hash",    key: "hash",    render: (r) => h("span", { class: "mono muted" }, shortHash(r.hash || r.digest || "")) },
        { label: "At",      key: "at",      render: (r) => h("span", { class: "muted" }, dateShort(toMs(r.at || r.time || r.timestamp))) },
      ],
      rows: data,
      onRowClick: (r) => openDetail(r),
      emptyText: "No entries match.",
    });
    host.appendChild(tbl.el);
  }

  function openDetail(entry) {
    sheet({
      title: `Entry #${entry.index ?? "—"}`,
      subtitle: entry.kind || "audit entry",
      body: h("div", { class: "stack" },
        h("div", { class: "json-viewer", html: prettyJson(entry) }),
      ),
      footer: [
        h("button", {
          class: "btn btn--accent btn--sm",
          onClick: () => verifyOne(entry),
        }, "Verify signature"),
      ],
    });
  }

  async function verifyAll() {
    toast.info("Verifying audit chain…");
    const r = await tryApi("/api/v4/audit/verify");
    if (r.ok && (r.data?.valid || r.data === true || r.data?.ok)) {
      toast.ok("Audit chain valid", r.data?.root ? shortHash(r.data.root) : undefined);
    } else {
      toast.err("Chain verification failed", r.data?.error || r.error?.message || "unknown");
    }
  }

  async function verifyOne(entry) {
    const r = await tryApi(`/api/v4/audit/verify`, { params: { index: entry.index } });
    if (r.ok) toast.ok("Signature valid");
    else toast.err("Invalid signature", r.error?.message);
  }

  return () => scope();
}

function kpi(label, value) {
  const el = h("div", { class: "kpi" },
    h("span", { class: "kpi__label" }, label),
    h("span", { class: "kpi__value" }, value),
  );
  const v = el.querySelector(".kpi__value");
  return { el, setValue(x) { v.textContent = x; } };
}

function kindClass(k) {
  const v = String(k || "").toLowerCase();
  if (v.includes("heal")) return "ok";
  if (v.includes("fail") || v.includes("error")) return "err";
  if (v.includes("warn") || v.includes("degr")) return "warn";
  if (v.includes("attest") || v.includes("sign")) return "violet";
  return "info";
}
function shortHash(s) { if (!s) return "—"; const str = String(s); return str.length > 12 ? str.slice(0, 6) + "…" + str.slice(-4) : str; }
function toMs(t) { if (!t) return null; if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? null : d; }
function prettyJson(o) {
  const json = JSON.stringify(o, null, 2);
  return json
    .replace(/(&|<|>)/g, (m) => ({ "&":"&amp;", "<":"&lt;", ">":"&gt;" }[m]))
    .replace(/"([^"]+)":/g, '<span class="json-key">"$1"</span>:')
    .replace(/: "([^"]*)"/g, ': <span class="json-str">"$1"</span>')
    .replace(/: (-?\d+(?:\.\d+)?)/g, ': <span class="json-num">$1</span>')
    .replace(/: (true|false)/g, ': <span class="json-bool">$1</span>')
    .replace(/: null/g, ': <span class="json-null">null</span>');
}
