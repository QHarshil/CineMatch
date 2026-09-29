import type { AssistantEvent, AssistantPick, ChatTurn, DoneEvent, RunStatus } from "./assistant-stream";

export interface ToolStep {
  id: string;
  tool: string;
  label: string;
  args: Record<string, unknown>;
  status: "running" | "done" | "error";
  count?: number;
  latencyMs?: number;
  retrieval?: string;
  titles?: string[];
  error?: string;
}

export type ExchangeStatus = "streaming" | RunStatus | "failed";

/** One prompt and everything the agent did to answer it. */
export interface Exchange {
  id: string;
  prompt: string;
  status: ExchangeStatus;
  steps: ToolStep[];
  message: string;
  picks: AssistantPick[];
  dropped: number;
  model?: string;
  promptVersion?: string;
  run?: DoneEvent;
  error?: { message: string; status?: number; resetsAt?: string };
}

export function newExchange(id: string, prompt: string): Exchange {
  return { id, prompt, status: "streaming", steps: [], message: "", picks: [], dropped: 0 };
}

/** Folds one stream event into the exchange. Pure, so it is easy to test. */
export function applyEvent(exchange: Exchange, event: AssistantEvent): Exchange {
  switch (event.type) {
    case "start":
      return { ...exchange, model: event.data.model, promptVersion: event.data.prompt_version };
    case "tool_call":
      return {
        ...exchange,
        steps: [
          ...exchange.steps,
          {
            id: event.data.id,
            tool: event.data.tool,
            label: event.data.label,
            args: event.data.args ?? {},
            status: "running",
          },
        ],
      };
    case "tool_result":
      return {
        ...exchange,
        steps: exchange.steps.map((step) =>
          step.id === event.data.id && step.status === "running"
            ? {
                ...step,
                status: event.data.error ? "error" : "done",
                count: event.data.count,
                latencyMs: event.data.latency_ms,
                retrieval: event.data.retrieval,
                titles: event.data.titles,
                error: event.data.error,
              }
            : step,
        ),
      };
    case "picks":
      return { ...exchange, message: event.data.message, picks: event.data.picks, dropped: event.data.dropped };
    case "message":
      return { ...exchange, message: event.data.text };
    case "error":
      return { ...exchange, error: { message: event.data.message } };
    case "done":
      return { ...exchange, status: event.data.status, run: event.data, model: exchange.model ?? event.data.model };
  }
}

/** Longest user turn the API accepts (backend/handlers/assistant.go). */
export const MAX_TURN_CHARS = 800;

/**
 * Rebuilds the conversation the API expects from finished exchanges. The
 * assistant side carries its message and the titles it picked, so follow-ups
 * like "something darker" have context.
 */
export function toTurns(history: Exchange[], prompt: string): ChatTurn[] {
  const turns: ChatTurn[] = [];
  for (const ex of history) {
    if (ex.status === "streaming" || ex.status === "failed") continue;
    turns.push({ role: "user", content: ex.prompt });
    const picked = ex.picks.map((p) => `${p.movie.title} (${p.movie.release_year})`).join(", ");
    const content = [ex.message, picked && `Picks: ${picked}`].filter(Boolean).join(" ");
    if (content) turns.push({ role: "assistant", content: content.slice(0, MAX_TURN_CHARS) });
  }
  turns.push({ role: "user", content: prompt });
  return turns;
}

/** Maps retrieval similarity to a coarse confidence label for the UI. */
export function confidenceOf(similarity: number | undefined): { label: string; level: number } {
  if (similarity == null) return { label: "From your ranking", level: 0 };
  // Query-to-title cosine similarities from text-embedding-3-small cluster
  // between 0.25 and 0.65, so the bands sit inside that range.
  if (similarity >= 0.55) return { label: "Strong match", level: 5 };
  if (similarity >= 0.5) return { label: "Strong match", level: 4 };
  if (similarity >= 0.4) return { label: "Good match", level: 3 };
  if (similarity >= 0.32) return { label: "Loose match", level: 2 };
  return { label: "Loose match", level: 1 };
}

export const TOOL_NAMES: Record<string, string> = {
  search_catalog: "Hybrid search",
  find_similar: "Similar titles",
  get_taste_profile: "Taste profile",
  get_recommendations: "Recommender",
};
