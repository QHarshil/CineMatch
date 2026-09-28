import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import { arrival, darkSeries } from "@/test/fixtures";
import { discoverTitles } from "@/lib/api";
import { BrowseContent } from "./browse-content";

vi.mock("@/lib/supabase-browser", () => ({
  createSupabaseBrowserClient: () => supabaseStub([arrival, darkSeries]),
}));

vi.mock("@/lib/api", () => ({
  discoverTitles: vi.fn(async () => ({
    retrieval: "hybrid",
    results: [
      { ...arrival, similarity: 0.52, semantic_rank: 1, keyword_rank: null, title_rank: null, score: 0.016 },
    ],
  })),
}));

describe("BrowseContent", () => {
  it("renders genre chips and the catalog grid", async () => {
    render(<BrowseContent genres={["Drama", "Mystery"]} searchQuery="" />);

    expect(screen.getByRole("button", { name: "Drama" })).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("Dark")).toBeInTheDocument();
  });

  it("runs a natural-language search and says how results matched", async () => {
    render(<BrowseContent genres={["Drama"]} searchQuery="slow-burn first contact" />);

    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("Matched by meaning, keywords, and title")).toBeInTheDocument();
    expect(discoverTitles).toHaveBeenCalledWith("slow-burn first contact", { limit: 40 });
    expect(screen.queryByRole("button", { name: "Drama" })).not.toBeInTheDocument();
  });
});
