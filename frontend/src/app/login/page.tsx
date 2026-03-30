"use client";

import { useState, type FormEvent } from "react";
import { useAuth } from "@/lib/auth-context";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export default function LoginPage() {
  const { signInWithMagicLink } = useAuth();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
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
      <div className="flex flex-col items-center justify-center py-24 gap-4 px-4">
        <h1 className="text-2xl font-bold">Check your email</h1>
        <p className="text-muted-foreground text-center max-w-md">
          We sent a magic link to <strong>{email}</strong>. Click it to sign in.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center py-24 gap-6 px-4">
      <div className="flex flex-col gap-2 text-center">
        <h1 className="text-2xl font-bold">Sign in to CineMatch</h1>
        <p className="text-muted-foreground">
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
        />
        <Button type="submit" disabled={submitting}>
          {submitting ? "Sending..." : "Send magic link"}
        </Button>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </form>
    </div>
  );
}
