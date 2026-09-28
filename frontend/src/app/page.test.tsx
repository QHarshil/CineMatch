import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import { arrival, darkSeries } from "@/test/fixtures";
import { discoverTitles } from "@/lib/api";
import HomePage from "./page";

vi.mock("@/lib/supabase-server", () => ({
  createSupabaseServerClient: async () => supabaseStub([arrival, darkSeries]),
}));

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({ user: null, session: null, loading: false }),
}));

vi.mock("@/lib/api", () => ({
  discoverTitles: vi.fn(),
}));

const inception = {
  ...arrival,
  id: "7f1c2a4e-0000-4000-8000-000000000003",
  title: "Inception",
  similarity: 0.515,
  semantic_rank: 1,
  keyword_rank: null,
  title_rank: null,
  score: 0.016,
};

describe("HomePage", () => {
  it("renders the hero, live demo results, and catalog rows", async () => {
    vi.mocked(discoverTitles).mockResolvedValue({ retrieval: "hybrid", results: [inception] });
    render(await HomePage());

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/describe a mood/i);
    expect(screen.getByLabelText("Describe what you want to watch")).toBeInTheDocument();
    expect(screen.getByText("Agent eval cases passed")).toBeInTheDocument();
    expect(screen.getByText(/1\s+Inception\s+cos 0\.52/)).toBeInTheDocument();
    expect(screen.getByText("Trending Now")).toBeInTheDocument();
    expect(screen.getAllByText("Arrival").length).toBeGreaterThan(0);
  });

  it("still renders when the API is down", async () => {
    vi.mocked(discoverTitles).mockRejectedValue(new Error("unreachable"));
    render(await HomePage());

    expect(screen.getAllByText(/try any description in the search bar/).length).toBeGreaterThan(0);
    expect(screen.queryByText(/cos 0\./)).not.toBeInTheDocument();
  });
});
