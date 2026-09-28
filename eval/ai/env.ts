/** Environment and service helpers for the evals. Reads the repo-root .env. */

import { createHmac } from "node:crypto";
import { mkdirSync, writeFileSync } from "node:fs";

try {
  process.loadEnvFile(new URL("../../.env", import.meta.url));
} catch {
  // Variables may come from the shell instead.
}

export function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required (set it in .env or the shell)`);
  return value;
}

export const supabaseUrl = () => requireEnv("SUPABASE_URL").replace(/\/$/, "");

/** Calls a Postgres function through PostgREST with the service key. */
export async function callRpc<T>(fn: string, payload: Record<string, unknown>): Promise<T> {
  const key = requireEnv("SUPABASE_SECRET_KEY");
  const res = await fetch(`${supabaseUrl()}/rest/v1/rpc/${fn}`, {
    method: "POST",
    headers: { apikey: key, Authorization: `Bearer ${key}`, "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`${fn}: HTTP ${res.status} ${await res.text()}`);
  return (await res.json()) as T;
}

/** Reads every matching row through PostgREST, paging past its 1,000-row cap. */
export async function selectRows<T>(table: string, query: string): Promise<T[]> {
  const key = requireEnv("SUPABASE_SECRET_KEY");
  const pageSize = 1000;
  const rows: T[] = [];
  for (let offset = 0; ; offset += pageSize) {
    const res = await fetch(`${supabaseUrl()}/rest/v1/${table}?${query}&order=id&limit=${pageSize}&offset=${offset}`, {
      headers: { apikey: key, Authorization: `Bearer ${key}` },
    });
    if (!res.ok) throw new Error(`${table}: HTTP ${res.status} ${await res.text()}`);
    const page = (await res.json()) as T[];
    rows.push(...page);
    if (page.length < pageSize) return rows;
  }
}

/** Embeds text with the model that embedded the catalog. */
export async function embed(text: string): Promise<number[]> {
  const res = await fetch("https://api.openai.com/v1/embeddings", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${requireEnv("OPENAI_API_KEY")}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ model: "text-embedding-3-small", input: text }),
  });
  if (!res.ok) throw new Error(`embeddings: HTTP ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { data: Array<{ embedding: number[] }> };
  return body.data[0].embedding;
}

/**
 * Signs a short-lived HS256 Supabase-style JWT for the eval user. The backend
 * accepts HS256 tokens signed with JWT_SECRET, so this only works against a
 * backend that shares the local secret.
 */
export function mintToken(userId: string, ttlSeconds = 3600): string {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  const header = encode({ alg: "HS256", typ: "JWT" });
  const claims = encode({ sub: userId, role: "authenticated", aud: "authenticated", iat: now, exp: now + ttlSeconds });
  const signature = createHmac("sha256", requireEnv("JWT_SECRET")).update(`${header}.${claims}`).digest("base64url");
  return `${header}.${claims}.${signature}`;
}

/** Writes a JSON report to eval/ai/results/ and returns its path. */
export function writeReport(name: string, report: unknown): string {
  const dir = new URL("./results/", import.meta.url);
  mkdirSync(dir, { recursive: true });
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const file = new URL(`${name}-${stamp}.json`, dir);
  writeFileSync(file, `${JSON.stringify(report, null, 2)}\n`);
  return file.pathname;
}
