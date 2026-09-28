import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { supabaseStub } from "@/test/supabase-stub";
import ForYouPage from "./page";

vi.mock("@/lib/supabase-browser", () => ({
  createSupabaseBrowserClient: () => supabaseStub([]),
}));

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({ user: null, session: null, loading: false }),
}));

describe("ForYouPage", () => {
  it("offers sign-in and demo taste profiles to signed-out visitors", () => {
    render(<ForYouPage />);

    expect(screen.getAllByRole("link", { name: /sign in/i }).length).toBeGreaterThan(0);
    expect(screen.getByText("Sci-fi & Thriller")).toBeInTheDocument();
  });
});
