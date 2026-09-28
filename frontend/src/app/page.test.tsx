import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import { arrival, darkSeries } from "@/test/fixtures";
import HomePage from "./page";

vi.mock("@/lib/supabase-server", () => ({
  createSupabaseServerClient: async () => supabaseStub([arrival, darkSeries]),
}));

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({ user: null, session: null, loading: false }),
}));

describe("HomePage", () => {
  it("renders the hero and the catalog rows", async () => {
    render(await HomePage());

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      /recommendations that learn/i,
    );
    expect(screen.getByText("Trending Now")).toBeInTheDocument();
    expect(screen.getAllByText("Arrival").length).toBeGreaterThan(0);
  });
});
