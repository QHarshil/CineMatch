"use client";

import { useState, useEffect, useRef, type FormEvent } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Input } from "@/components/ui/input";
import { MailCheck } from "lucide-react";

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
      <div className="flex min-h-[70vh] flex-col items-center justify-center px-4 py-20">
        <div className="w-full max-w-md border border-border bg-background p-8 text-center sm:p-10">
          <span className="mx-auto mb-5 grid size-12 place-items-center border border-primary text-primary">
            <MailCheck className="size-6" strokeWidth={1.5} />
          </span>
          <p className="eyebrow text-primary">Magic link sent</p>
          <h1 className="mt-3 font-heading text-3xl font-semibold uppercase tracking-tight">
            Check your inbox
          </h1>
          <p className="mt-3 font-serif leading-relaxed text-muted-foreground">
            If an account exists for{" "}
            <strong className="text-foreground">{email}</strong>, a one-tap
            sign-in link is on its way. It expires in a few minutes.
          </p>
          {cooldownLeft > 0 ? (
            <p className="mt-6 font-mono text-xs text-muted-foreground">
              You can request another link in {cooldownLeft}s
            </p>
          ) : (
            <button
              onClick={() => setSent(false)}
              className="eyebrow mt-6 text-primary transition-colors hover:text-primary/80"
            >
              Send another link
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[70vh] flex-col items-center justify-center px-4 py-20">
      <div className="w-full max-w-md border border-border bg-background p-8 sm:p-10">
        <p className="eyebrow text-primary">Passwordless sign in</p>
        <h1 className="mt-3 font-heading text-3xl font-semibold uppercase tracking-tight">
          Welcome to CineMatch
        </h1>
        <p className="mt-3 font-serif leading-relaxed text-muted-foreground">
          Enter your email and we will send a one-tap magic link. No passwords to
          remember, ever.
        </p>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-3">
          <Input
            type="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="h-12 rounded-none border-border bg-surface text-foreground placeholder:text-muted-foreground"
          />
          <button
            type="submit"
            disabled={submitting || cooldownLeft > 0}
            className="eyebrow h-12 bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
          >
            {submitting
              ? "Sending..."
              : cooldownLeft > 0
                ? `Wait ${cooldownLeft}s`
                : "Send magic link"}
          </button>
          {error && (
            <p className="font-serif text-sm text-destructive">{error}</p>
          )}
        </form>

        <p className="mt-6 border-t border-border pt-4 font-mono text-[11px] leading-relaxed text-muted-foreground">
          We store your email only as a SHA-256 hash. Zero plaintext PII.
        </p>
      </div>
    </div>
  );
}
