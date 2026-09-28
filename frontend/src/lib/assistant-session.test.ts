import { describe, expect, it } from "vitest";
import { applyEvent, confidenceOf, newExchange, toTurns, type Exchange } from "./assistant-session";
import type { AssistantPick } from "./assistant-stream";
import { arrival } from "@/test/fixtures";

const pick: AssistantPick = { movie: arrival, reason: "First contact, told patiently.", similarity: 0.52, source: "search_catalog" };

describe("applyEvent", () => {
  it("builds an exchange from a full stream", () => {
    let ex = newExchange("e1", "slow-burn sci-fi");
    ex = applyEvent(ex, { type: "start", data: { model: "qwen3:8b", prompt_version: "assistant-v1" } });
    ex = applyEvent(ex, { type: "tool_call", data: { id: "c1", tool: "search_catalog", label: "Searching", args: { query: "sci-fi" } } });
    expect(ex.steps[0].status).toBe("running");

    ex = applyEvent(ex, { type: "tool_result", data: { id: "c1", tool: "search_catalog", count: 8, latency_ms: 120, retrieval: "hybrid" } });
    expect(ex.steps[0]).toMatchObject({ status: "done", count: 8, latencyMs: 120, retrieval: "hybrid" });

    ex = applyEvent(ex, { type: "picks", data: { message: "One pick.", picks: [pick], dropped: 1 } });
    ex = applyEvent(ex, {
      type: "done",
      data: { run_id: "r1", status: "picks", model: "qwen3:8b", usage: { input_tokens: 900, output_tokens: 90 }, latency_ms: 4000, remaining_today: 7 },
    });
    expect(ex).toMatchObject({ status: "picks", model: "qwen3:8b", message: "One pick.", dropped: 1 });
    expect(ex.run?.run_id).toBe("r1");
  });

  it("marks a failed tool call", () => {
    let ex = newExchange("e1", "x");
    ex = applyEvent(ex, { type: "tool_call", data: { id: "c1", tool: "find_similar", label: "Finding", args: {} } });
    ex = applyEvent(ex, { type: "tool_result", data: { id: "c1", tool: "find_similar", count: 0, latency_ms: 3, error: "unknown ref" } });
    expect(ex.steps[0]).toMatchObject({ status: "error", error: "unknown ref" });
  });
});

describe("toTurns", () => {
  it("replays finished exchanges with the titles picked, and skips failed ones", () => {
    const done: Exchange = { ...newExchange("e1", "slow-burn sci-fi"), status: "picks", message: "Try this.", picks: [pick] };
    const failed: Exchange = { ...newExchange("e2", "broken"), status: "failed" };
    expect(toTurns([done, failed], "something darker")).toEqual([
      { role: "user", content: "slow-burn sci-fi" },
      { role: "assistant", content: "Try this. Picks: Arrival (2016)" },
      { role: "user", content: "something darker" },
    ]);
  });
});

describe("confidenceOf", () => {
  it.each([
    [0.6, "Strong match"],
    [0.45, "Good match"],
    [0.3, "Loose match"],
    [undefined, "From your ranking"],
  ])("labels similarity %s as %s", (similarity, label) => {
    expect(confidenceOf(similarity).label).toBe(label);
  });

  it("keeps the meter between one and five bars", () => {
    expect(confidenceOf(0.05).level).toBe(1);
    expect(confidenceOf(0.99).level).toBe(5);
  });
});
