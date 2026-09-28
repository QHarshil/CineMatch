import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import { arrival } from "@/test/fixtures";
import HowItWorksPage from "./page";

vi.mock("@/lib/supabase-server", () => ({
  createSupabaseServerClient: async () => supabaseStub([arrival]),
}));

describe("HowItWorksPage", () => {
  it("renders the deep dive with the eval table", async () => {
    render(await HowItWorksPage());

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/how cinematch builds recommendations/i);
    expect(screen.getAllByText(/LambdaMART/i).length).toBeGreaterThan(0);
  });
});
