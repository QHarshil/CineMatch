"use client";

import { useState, type FormEvent } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Input } from "@/components/ui/input";

export function LoginForm() {
  const { signInWithMagicLink } = useAuth();
  const searchParams = useSearchParams();
  const callbackError = searchParams.get("error");
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(
    callbackError ? "Magic link expired or invalid. Please try again." : null
  );
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signInWithMagicLink(email);
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign in failed");
    } finally {
      setSubmitting(false);
    }
  }

  if (sent) {
    return (
      <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4 px-4">
        <h1 className="font-heading text-3xl font-semibold">
          Check your email
        </h1>
        <p className="text-muted-foreground text-sm text-center max-w-md">
          We sent a magic link to <strong className="text-foreground">{email}</strong>.
          Click it to sign in.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-8 px-4">
      <div className="flex flex-col gap-2 text-center">
        <h1 className="font-heading text-3xl font-semibold">Sign in</h1>
        <p className="text-muted-foreground text-sm">
          No password needed. We will email you a magic link.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3 w-full max-w-sm">
        <Input
          type="email"
          placeholder="you@example.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="h-12 bg-surface border-border text-foreground placeholder:text-muted-foreground"
        />
        <button
          type="submit"
          disabled={submitting}
          className="h-12 bg-gold text-background text-sm font-medium hover:bg-gold-dim transition-colors duration-200 disabled:opacity-50"
        >
          {submitting ? "Sending..." : "Send magic link"}
        </button>
        {error && (
          <p className="text-sm text-destructive text-center">{error}</p>
        )}
      </form>
    </div>
  );
}
