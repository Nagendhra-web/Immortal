// certificates.js — post-quantum attestations & proofs.

import { h, dateShort, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { table } from "../components/table.js";
import { sheet } from "../components/sheet.js";
import { toast } from "../components/toast.js";
import { dialog } from "../components/dialog.js";
import { parseFilter } from "../lib/filter-dsl.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Certificates"),
      h("div", { class: "view-header__subtitle" },
        "The paper trail: PQ audit signatures, attestations, twin checkpoints, formal proofs. Each is independently verifiable."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--secondary btn--sm", onClick: exportBundle }, "⬇ Export bundle"),
      h("button", { class: "btn btn--danger btn--sm", onClick: rotateKeys }, "↻ Rotate keys"),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const filterCard = h("div", { class: "filter-bar" },
    h("input", {
      class: "input input--mono", style: { width: "340px" },
      placeholder: "kind=attest or subject~payments",
      dataset: { role: "view-filter" },
      onInput: (e) => { pred = safe(parseFilter, e.target.value); render(); },
    }),
    h("div", { class: "segmented", id: "cr-kind" },
      h("button", { "aria-pressed": "true",  dataset: { v: "all" } },              "All"),
      h("button", { "aria-pressed": "false", dataset: { v: "pqaudit" } },          "PQ audit"),
      h("button", { "aria-pressed": "false", dataset: { v: "attest" } },           "Attest"),
      h("button", { "aria-pressed": "false", dataset: { v: "twin-checkpoint" } },  "Twin"),
      h("button", { "aria-pressed": "false", dataset: { v: "formal-proof" } },     "Formal"),
    ),
  );
  body.appendChild(filterCard);

  const tableHost = h("div", { id: "cr-table" });
  body.appendChild(tableHost);

  let rows = [], pred = () => true, kind = "all";

  root.querySelectorAll("#cr-kind button").forEach((b) => {
    b.addEventListener("click", () => {
      root.querySelectorAll("#cr-kind button").forEach((x) => x.setAttribute("aria-pressed", "false"));
      b.setAttribute("aria-pressed", "true");
      kind = b.dataset.v;
      render();
    });
  });

  refresh();
  const sc = cmdk.scope("certs", [
    { id: "cert:rotate", group: "Actions", title: "Rotate PQ keys", run: rotateKeys },
    { id: "cert:export", group: "Actions", title: "Export certificate bundle", run: exportBundle },
  ]);

  async function refresh() {
    const [entries, root2] = await Promise.all([
      tryApi("/api/v4/audit/entries", { params: { kind: "cert", limit: 500 } }),
      tryApi("/api/v4/audit/merkle-root"),
    ]);
    rows = entries.ok ? (entries.data?.entries || entries.data || [])
                      : demoCerts();
    if (!rows.length) rows = demoCerts();
    render();
  }

  function render() {
    const filtered = rows.filter((r) => (kind === "all" || (r.kind || "").includes(kind)) && pred(r));
    tableHost.innerHTML = "";
    const tbl = table({
      columns: [
        { label: "Kind",   key: "kind", render: (r) =>
          h("span", { class: `badge badge--${kindBadge(r.kind)}` }, r.kind || "cert") },
        { label: "Subject",  key: "subject", value: (r) => r.subject || r.service || r.target },
        { label: "Issued",   key: "issued_at", render: (r) => h("span", { class: "muted" }, dateShort(toMs(r.issued_at || r.at))) },
        { label: "Expires",  key: "expires_at", render: (r) =>
          h("span", { class: expired(r) ? "badge badge--err" : "muted" }, r.expires_at ? dateShort(toMs(r.expires_at)) : "—") },
        { label: "Signer",   key: "signer", render: (r) => h("span", { class: "mono muted" }, (r.signer || r.signed_by || "—")) },
        { label: "Hash",     key: "hash", render: (r) => h("span", { class: "mono" }, short(r.hash || r.id)) },
        { label: "Status",   key: "status", render: (r) =>
          h("span", { class: `badge badge--${r.valid === false ? "err" : "ok"}` }, r.valid === false ? "invalid" : "valid") },
      ],
      rows: filtered,
      onRowClick: openDetail,
    });
    tableHost.appendChild(tbl.el);
  }

  function openDetail(r) {
    sheet({
      title: r.subject || r.target || "Certificate",
      subtitle: `${r.kind || "cert"} · ${short(r.hash || r.id)}`,
      body: h("div", { class: "stack" },
        h("div", { class: "row" },
          h("span", { class: `badge badge--${r.valid === false ? "err" : "ok"}` }, r.valid === false ? "INVALID" : "VALID"),
          r.expires_at ? h("span", { class: "muted" }, "expires " + relTime(toMs(r.expires_at))) : null,
          h("div", { style: { flex: 1 } }),
          h("button", { class: "btn btn--accent btn--sm", onClick: () => verify(r) }, "✓ Verify now"),
        ),
        h("h3", {}, "Certificate body"),
        h("div", { class: "json-viewer", html: prettyJson(r) }),
        r.signature ? h("div", { class: "stack" },
          h("h3", {}, "Signature"),
          h("div", { class: "mono", style: { background: "var(--bg)", border: "1px solid var(--border)", borderRadius: "var(--r-md)", padding: "12px", wordBreak: "break-all", fontSize: "11px", color: "var(--text-muted)" } }, r.signature),
        ) : null,
      ),
    });
  }

  async function verify(r) {
    toast.info("Verifying…");
    const res = await tryApi("/api/v4/audit/verify", { params: { id: r.id || r.hash } });
    if (res.ok && (res.data?.valid !== false)) toast.ok("Signature valid", short(r.hash || r.id));
    else toast.err("Invalid signature", res.data?.error || res.error?.message);
  }

  async function exportBundle() {
    toast.info("Bundling…");
    const blob = new Blob([JSON.stringify({ rows, exported_at: Date.now() }, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = h("a", { href: url, download: "immortal-certificates.json" });
    document.body.appendChild(a); a.click(); a.remove();
    URL.revokeObjectURL(url);
    toast.ok("Bundle exported", `${rows.length} entries`);
  }

  async function rotateKeys() {
    const dlg = dialog({
      title: "Rotate post-quantum signing keys?",
      body: h("div", {},
        h("p", {}, "All new attestations will be signed by the new keypair. Old certificates remain verifiable against their original public key."),
        h("p", { class: "muted" }, "This is an irreversible, audit-logged operation."),
      ),
      footer: [
        h("button", { class: "btn btn--ghost btn--sm", onClick: () => dlg.close() }, "Cancel"),
        h("button", { class: "btn btn--danger btn--sm", onClick: async () => {
          dlg.close();
          toast.info("Rotating keys…");
          const r = await tryApi("/api/v4/audit/verify", { method: "POST", body: { op: "rotate" } });
          if (r.ok) toast.ok("Rotated", r.data?.fingerprint || "new fingerprint stored");
          else toast.warn("Rotation endpoint unavailable", "(demo only)");
        } }, "Rotate"),
      ],
    });
  }

  return () => sc();
}

function kindBadge(k) {
  const v = String(k || "").toLowerCase();
  if (v.includes("pqaudit")) return "violet";
  if (v.includes("attest"))  return "info";
  if (v.includes("twin"))    return "lime";
  if (v.includes("formal"))  return "pink";
  return "info";
}
function expired(r) { return r.expires_at && toMs(r.expires_at) < Date.now(); }
function short(s) { if (!s) return "—"; const str = String(s); return str.length > 14 ? str.slice(0,8) + "…" + str.slice(-4) : str; }
function toMs(t) { if (!t) return null; if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? null : d; }
function safe(fn, arg) { try { return fn(arg); } catch { return () => true; } }
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
function demoCerts() {
  const kinds = ["pqaudit", "attest", "twin-checkpoint", "formal-proof"];
  const subjects = ["engine.boot", "payments@v4.2.0", "billing.checkpoint.2026-04-18T12:00", "SAFETY-001", "twin.state.hash", "rest.attest.72"];
  return Array.from({ length: 24 }, (_, i) => ({
    id: "crt_" + (Math.random() * 1e9 | 0).toString(16),
    hash: randomHex(64),
    kind: kinds[i % kinds.length],
    subject: subjects[i % subjects.length] + " #" + i,
    issued_at: Date.now() - (i + 1) * 3600_000,
    expires_at: Date.now() + (30 - i) * 86400_000,
    signer: "pq-dilithium-3:" + randomHex(10),
    signature: randomHex(512),
    valid: i !== 7,
  }));
}
function randomHex(n) { let s = ""; const c = "0123456789abcdef"; for (let i = 0; i < n; i++) s += c[Math.random() * 16 | 0]; return s; }
