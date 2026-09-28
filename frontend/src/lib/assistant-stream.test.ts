import { describe, expect, it } from "vitest";
import { readAssistantStream, splitEvents, toAssistantEvent } from "./assistant-stream";

function streamOf(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
}

const movie = { id: "m1", title: "Oldboy", genres: [], release_year: 2003 };

describe("splitEvents", () => {
  it("returns complete events and keeps the unfinished tail", () => {
    const { events, rest } = splitEvents('event: start\ndata: {"model":"m"}\n\nevent: mess');
    expect(events).toEqual([{ event: "start", data: '{"model":"m"}' }]);
    expect(rest).toBe("event: mess");
  });

  it("handles CRLF line endings", () => {
    const { events } = splitEvents('event: message\r\ndata: {"text":"hi"}\r\n\r\n');
    expect(events).toEqual([{ event: "message", data: '{"text":"hi"}' }]);
  });
});

describe("toAssistantEvent", () => {
  it.each([
    ["unknown event types", { event: "debug", data: "{}" }],
    ["invalid JSON", { event: "message", data: "{" }],
    ["missing fields", { event: "tool_call", data: '{"id":"c1"}' }],
    ["picks without movie ids", { event: "picks", data: '{"message":"m","dropped":0,"picks":[{"reason":"r"}]}' }],
  ])("rejects %s", (_name, raw) => {
    expect(toAssistantEvent(raw)).toBeNull();
  });

  it("accepts a valid picks event", () => {
    const data = JSON.stringify({ message: "Two picks", dropped: 1, picks: [{ movie, reason: "r", source: "search_catalog" }] });
    const event = toAssistantEvent({ event: "picks", data });
    expect(event?.type).toBe("picks");
  });
});

describe("readAssistantStream", () => {
  it("yields events split across arbitrary chunk boundaries", async () => {
    const body = [
      'event: start\ndata: {"model":"qwen3:8b","prompt_version":"assistant-v1"}\n\n',
      'event: tool_call\ndata: {"id":"c1","tool":"search_catalog","lab',
      'el":"Searching","args":{}}\n\nevent: tool_result\ndata: {"id":"c1","tool":"search_catalog","count":8,"latency_ms":90}\n',
      '\nevent: done\ndata: {"run_id":"r1","status":"picks","model":"m","usage":{"input_tokens":1,"output_tokens":1},"latency_ms":900,"remaining_today":3}',
    ];
    const types: string[] = [];
    for await (const event of readAssistantStream(streamOf(body))) types.push(event.type);
    expect(types).toEqual(["start", "tool_call", "tool_result", "done"]);
  });
});
