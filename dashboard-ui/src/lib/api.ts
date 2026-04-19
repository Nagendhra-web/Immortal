async function j<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
  });
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}

export const api = {
  health: () => j<any>("/api/health"),
  events: (limit = 20) => j<any>(`/api/events?limit=${limit}`),
  status: () => j<any>("/api/status"),
  dependencies: () => j<any>("/api/dependencies"),
  recommendations: () => j<any>("/api/recommendations"),
  auditVerify: () => j<any>("/api/v4/audit/verify"),
  auditEntries: (limit = 10) => j<any>(`/api/v4/audit/entries?limit=${limit}`),
  auditLegacy: (limit = 10) => j<any>(`/api/audit?limit=${limit}`),
  topologySnapshot: () => j<any>("/api/v5/topology/snapshot"),
  agenticRun: (ev: any) => j<any>("/api/v4/agentic/run", { method: "POST", body: JSON.stringify(ev) }),
  formalCheck: (body: any) => j<any>("/api/v4/formal/check", { method: "POST", body: JSON.stringify(body) }),
  nlplan: (body: any) => j<any>("/api/v5/nlplan/compile", { method: "POST", body: JSON.stringify(body) }),
  causalRootCause: (body: any) => j<any>("/api/v4/causal/root-cause", { method: "POST", body: JSON.stringify(body) }),
  pcmci: (body: any) => j<any>("/api/v5/causal/pcmci", { method: "POST", body: JSON.stringify(body) }),
};
