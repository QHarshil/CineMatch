"use client";

import { useState, useEffect, useRef, type FormEvent } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Input } from "@/components/ui/input";

const COOLDOWN_SECONDS = 60;

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
  const [cooldownLeft, setCooldownLeft] = useState(0);
  const cooldownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    return () => {
      if (cooldownRef.current) clearInterval(cooldownRef.current);
    };
  }, []);

  function startCooldown() {
    setCooldownLeft(COOLDOWN_SECONDS);
    cooldownRef.current = setInterval(() => {
      setCooldownLeft((prev) => {
        if (prev <= 1) {
          if (cooldownRef.current) clearInterval(cooldownRef.current);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signInWithMagicLink(email);
      setSent(true);
      startCooldown();
    } catch (err) {
      // Don't reveal whether email exists — always show generic message
      // unless it's clearly a client-side error.
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
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
          If an account exists for <strong className="text-foreground">{email}</strong>,
          we sent a magic link. Click it to sign in.
        </p>
        {cooldownLeft > 0 ? (
          <p className="text-xs text-muted-foreground">
            You can request another link in {cooldownLeft}s
          </p>
        ) : (
          <button
            onClick={() => setSent(false)}
            className="text-sm text-gold hover:text-gold-dim transition-colors"
          >
            Send another link
          </button>
        )}
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
          disabled={submitting || cooldownLeft > 0}
          className="h-12 bg-gold text-background text-sm font-medium hover:bg-gold-dim transition-colors duration-200 disabled:opacity-50"
        >
          {submitting
            ? "Sending..."
            : cooldownLeft > 0
              ? `Wait ${cooldownLeft}s`
              : "Send magic link"}
        </button>
        {error && (
          <p className="text-sm text-destructive text-center">{error}</p>
        )}
      </form>
    </div>
  );
}
