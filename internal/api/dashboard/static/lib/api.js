// api.js — thin fetch wrapper. Aborts on caller signal. JSON by default.

export async function api(path, { params, method = "GET", body, signal, headers } = {}) {
  const url = new URL(path, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v === null || v === undefined || v === "") continue;
      url.searchParams.set(k, String(v));
    }
  }
  const init = {
    method,
    signal,
    headers: {
      "Accept": "application/json",
      ...(body ? { "Content-Type": "application/json" } : {}),
      ...(headers || {}),
    },
  };
  if (body !== undefined) init.body = typeof body === "string" ? body : JSON.stringify(body);

  let res;
  try {
    res = await fetch(url.toString(), init);
  } catch (err) {
    if (err.name === "AbortError") throw err;
    throw new ApiError("network", 0, String(err));
  }
  const text = await res.text();
  let data = null;
  if (text) {
    try { data = JSON.parse(text); } catch { data = text; }
  }
  if (!res.ok) throw new ApiError(data?.error || res.statusText, res.status, text);
  return data;
}

export class ApiError extends Error {
  constructor(msg, status, raw) {
    super(msg);
    this.status = status;
    this.raw = raw;
  }
}

// tryApi — returns {ok, data} without throwing, useful for dashboards where
// some endpoints may not be enabled on this build.
export async function tryApi(path, opts) {
  try {
    const data = await api(path, opts);
    return { ok: true, data };
  } catch (err) {
    if (err.name === "AbortError") return { ok: false, aborted: true };
    return { ok: false, error: err, status: err.status };
  }
}
