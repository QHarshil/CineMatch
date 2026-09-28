import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LoginForm } from "./login-form";

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({
    user: null,
    session: null,
    loading: false,
    signInWithMagicLink: vi.fn(),
  }),
}));

describe("LoginForm", () => {
  it("asks for an email to send the magic link to", () => {
    render(<LoginForm />);
    expect(screen.getByRole("textbox")).toHaveAttribute("type", "email");
  });
});
