/**
 * Typed client for the POST /assistant server-sent event stream.
 *
 * Dependency-free so the offline eval runner (eval/ai) imports this same file
 * and exercises the parser the UI uses.
 */

import type { Movie } from "../types/movie";

export interface ToolCallEvent {
  id: string;
  tool: string;
  label: string;
  args: Record<string, unknown>;
}

export interface ToolResultEvent {
  id: string;
  tool: string;
  count: number;
  latency_ms: number;
  retrieval?: string;
  titles?: string[];
  error?: string;
}

export interface AssistantPick {
  movie: Movie;
  reason: string;
  /** Retrieval similarity behind the pick, shown as a confidence signal. */
  similarity?: number;
  /** The tool that surfaced the title. */
  source: string;
}

export interface PicksEvent {
  message: string;
  picks: AssistantPick[];
  /** Picks the server rejected because no tool returned them. */
  dropped: number;
}

export type RunStatus = "picks" | "answered" | "fallback" | "error";

export interface DoneEvent {
  run_id: string;
  status: RunStatus;
  model: string;
  usage: { input_tokens: number; output_tokens: number };
  latency_ms: number;
  remaining_today: number;
}

export type AssistantEvent =
  | { type: "start"; data: { model: string; prompt_version: string } }
  | { type: "tool_call"; data: ToolCallEvent }
  | { type: "tool_result"; data: ToolResultEvent }
  | { type: "picks"; data: PicksEvent }
  | { type: "message"; data: { text: string } }
  | { type: "error"; data: { code: string; message: string } }
  | { type: "done"; data: DoneEvent };

export interface ChatTurn {
  role: "user" | "assistant";
  content: string;
}

interface RawEvent {
  event: string;
  data: string;
}

/**
 * Splits buffered stream text into complete events. Returns the events and
 * the unfinished tail to prepend to the next chunk.
 */
export function splitEvents(buffer: string): { events: RawEvent[]; rest: string } {
  const normalized = buffer.replace(/\r\n/g, "\n");
  const blocks = normalized.split("\n\n");
  const rest = blocks.pop() ?? "";
  const events: RawEvent[] = [];
  for (const block of blocks) {
    let event = "message";
    const data: string[] = [];
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
    }
    if (data.length > 0) events.push({ event, data: data.join("\n") });
  }
  return { events, rest };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

const REQUIRED_FIELDS: Record<AssistantEvent["type"], Record<string, string>> = {
  start: { model: "string" },
  tool_call: { id: "string", tool: "string", label: "string" },
  tool_result: { id: "string", tool: "string", count: "number" },
  picks: { message: "string", picks: "array", dropped: "number" },
  message: { text: "string" },
  error: { code: "string", message: "string" },
  done: { run_id: "string", status: "string", latency_ms: "number" },
};

/**
 * Validates one raw event against the expected shape. Unknown event types and
 * malformed payloads return null, so a server change cannot crash the UI.
 */
export function toAssistantEvent(raw: RawEvent): AssistantEvent | null {
  if (!(raw.event in REQUIRED_FIELDS)) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw.data);
  } catch {
    return null;
  }
  if (!isRecord(parsed)) return null;
  const required = REQUIRED_FIELDS[raw.event as AssistantEvent["type"]];
  for (const [field, kind] of Object.entries(required)) {
    const value = parsed[field];
    const ok = kind === "array" ? Array.isArray(value) : typeof value === kind;
    if (!ok) return null;
  }
  if (raw.event === "picks") {
    const picks = parsed.picks as unknown[];
    if (!picks.every((p) => isRecord(p) && isRecord(p.movie) && typeof p.movie.id === "string")) {
      return null;
    }
  }
  return { type: raw.event, data: parsed } as AssistantEvent;
}

/** Reads a response body and yields validated events as they arrive. */
export async function* readAssistantStream(body: ReadableStream<Uint8Array>): AsyncGenerator<AssistantEvent> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      const { events, rest } = splitEvents(done ? `${buffer}\n\n` : buffer);
      buffer = rest;
      for (const raw of events) {
        const event = toAssistantEvent(raw);
        if (event) yield event;
      }
      if (done) return;
    }
  } finally {
    reader.releaseLock();
  }
}

/** Thrown for non-streaming error responses (validation, auth, quota). */
export class AssistantRequestError extends Error {
  readonly status: number;
  readonly resetsAt?: string;

  constructor(status: number, message: string, resetsAt?: string) {
    super(message);
    this.name = "AssistantRequestError";
    this.status = status;
    this.resetsAt = resetsAt;
  }
}

/**
 * Starts an assistant run and yields its events. The JWT is sent in both
 * Authorization and X-Authorization for proxies that strip the former.
 */
export async function* streamAssistant(options: {
  apiBase: string;
  token: string;
  turns: ChatTurn[];
  signal?: AbortSignal;
}): AsyncGenerator<AssistantEvent> {
  const res = await fetch(`${options.apiBase}/assistant`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${options.token}`,
      "X-Authorization": `Bearer ${options.token}`,
    },
    body: JSON.stringify({ messages: options.turns }),
    signal: options.signal,
  });
  if (!res.ok || !res.body) {
    let message = `Assistant request failed (${res.status})`;
    let resetsAt: string | undefined;
    try {
      const body: unknown = await res.json();
      if (isRecord(body)) {
        if (typeof body.error === "string") message = body.error;
        if (typeof body.resets_at === "string") resetsAt = body.resets_at;
      }
    } catch {
      // Non-JSON error body: keep the status message.
    }
    throw new AssistantRequestError(res.status, message, resetsAt);
  }
  yield* readAssistantStream(res.body);
}
