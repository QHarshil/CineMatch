import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import { arrival, darkSeries } from "@/test/fixtures";
import { BrowseContent } from "./browse-content";

vi.mock("@/lib/supabase-browser", () => ({
  createSupabaseBrowserClient: () => supabaseStub([arrival, darkSeries]),
}));

describe("BrowseContent", () => {
  it("renders genre chips and the catalog grid", async () => {
    render(<BrowseContent genres={["Drama", "Mystery"]} searchQuery="" />);

    expect(screen.getByRole("button", { name: "Drama" })).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("Dark")).toBeInTheDocument();
  });
});
