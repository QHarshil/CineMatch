import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Header } from "./header";

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({ user: null, session: null, loading: false, signOut: vi.fn() }),
}));

describe("Header", () => {
  it("shows the primary navigation and a sign-in link", () => {
    render(<Header />);

    for (const label of ["Browse", "For You", "How It Works"]) {
      expect(screen.getAllByRole("link", { name: label }).length).toBeGreaterThan(0);
    }
    expect(screen.getAllByRole("link", { name: /sign in/i }).length).toBeGreaterThan(0);
  });
});
