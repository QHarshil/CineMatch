import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { arrival } from "@/test/fixtures";
import { newExchange, type Exchange } from "@/lib/assistant-session";
import { ExchangeView } from "./exchange-view";

vi.mock("@/components/toast", () => ({ useToast: () => ({ showToast: vi.fn() }) }));

const finished: Exchange = {
  ...newExchange("e1", "A slow-burn sci-fi that makes me think"),
  status: "picks",
  model: "qwen3:8b",
  message: "Two patient first-contact films.",
  steps: [{ id: "c1", tool: "search_catalog", label: 'Searching for "slow-burn sci-fi"', args: {}, status: "done", count: 8, latencyMs: 140 }],
  picks: [{ movie: arrival, reason: "A linguist decodes an alien language.", similarity: 0.54, source: "search_catalog" }],
  dropped: 1,
  run: { run_id: "r1", status: "picks", model: "qwen3:8b", usage: { input_tokens: 5000, output_tokens: 200 }, latency_ms: 9400, remaining_today: 6 },
};

describe("ExchangeView", () => {
  it("shows the tool steps, grounded picks, and run summary", () => {
    render(<ExchangeView exchange={finished} token="t" />);

    expect(screen.getByText('Searching for "slow-burn sci-fi"')).toBeInTheDocument();
    expect(screen.getByText("8 results · 140 ms")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Arrival" })).toBeInTheDocument();
    expect(screen.getByText("Strong match")).toBeInTheDocument();
    expect(screen.getByText(/1 grounded picks, 1 ungrounded blocked/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Like Arrival" })).toBeInTheDocument();
  });

  it("explains a daily-limit error without offering a retry", () => {
    const limited: Exchange = {
      ...newExchange("e2", "one more"),
      status: "failed",
      error: { message: "daily limit of 8 assistant requests reached", status: 429, resetsAt: "2026-09-29T00:00:00Z" },
    };
    render(<ExchangeView exchange={limited} onRetry={vi.fn()} />);

    expect(screen.getByRole("alert")).toHaveTextContent(/daily limit of 8/);
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });
});
