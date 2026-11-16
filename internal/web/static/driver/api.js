// Requests to the server's driver API. One that fails throws an ApiError
// with the status and the server's error code.

import { API, S } from "./state.js";

export class ApiError extends Error {
  constructor(status, body) {
    super((body && body.error && body.error.message) || "Request failed");
    this.status = status;
    this.code = body && body.error && body.error.code;
    this.field = body && body.error && body.error.field;
  }
}

async function gzip(text) {
  if (!("CompressionStream" in window)) return null;
  const stream = new Blob([text]).stream().pipeThrough(new CompressionStream("gzip"));
  return new Response(stream).arrayBuffer();
}

export async function api(path, { method = "GET", body, auth = true, compress = false, timeout = 30000 } = {}) {
  const headers = { Accept: "application/json" };
  let payload;
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
    if (compress && payload.length > 2048) {
      const gz = await gzip(payload);
      if (gz) {
        payload = gz;
        headers["Content-Encoding"] = "gzip";
      }
    }
  }
  if (auth && S.token) headers.Authorization = "Bearer " + S.token;
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeout);
  try {
    const res = await fetch(API + path, { method, headers, body: payload, cache: "no-store", signal: ctl.signal });
    let data = null;
    try {
      data = await res.json();
    } catch (e) {
      if (ctl.signal.aborted) throw e; // empty bodies are fine, cut-off ones aren't
    }
    if (!res.ok) throw new ApiError(res.status, data);
    return data;
  } finally {
    clearTimeout(timer);
  }
}
