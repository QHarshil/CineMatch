"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowUpRight, RotateCcw } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useAssistant } from "@/hooks/use-assistant";
import { SplitHeading } from "@/components/motion/split-heading";
import { AgentTrace } from "./agent-trace";
import { Composer } from "./composer";
import { ExchangeView } from "./exchange-view";

const SUGGESTIONS = [
  { group: "A mood", prompts: ["A slow-burn sci-fi that makes me think", "Something cozy and funny for a rainy night"] },
  { group: "Like a title", prompts: ["Something like Parasite, but a series", "Movies like Prisoners"] },
  { group: "From your taste", prompts: ["Recommend something based on what I like", "A series that matches the movies I've liked"] },
];

function Suggestions({ onPick }: { onPick: (prompt: string) => void }) {
  return (
    <div className="mt-10 grid gap-px border border-border bg-border sm:grid-cols-3">
      {SUGGESTIONS.map((s) => (
        <div key={s.group} className="bg-background p-4">
          <p className="eyebrow text-primary">{s.group}</p>
          <ul className="mt-3 space-y-2">
            {s.prompts.map((prompt) => (
              <li key={prompt}>
                <button
                  type="button"
                  onClick={() => onPick(prompt)}
                  className="group flex w-full items-start justify-between gap-2 text-left font-serif text-sm leading-snug text-foreground transition-colors hover:text-primary"
                >
                  {prompt}
                  <ArrowUpRight className="mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

function SignedOut() {
  const { signInAsGuest } = useAuth();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function tryAsGuest() {
    setPending(true);
    setError(null);
    try {
      await signInAsGuest();
    } catch {
      setError("Guest access is not available right now. Sign in with email instead.");
      setPending(false);
    }
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col items-center px-6 py-24 text-center">
      <p className="eyebrow text-primary">Grounded assistant</p>
      <SplitHeading
        as="h1"
        immediate
        text="Ask for a mood. Get titles that exist."
        className="mt-5 font-heading text-4xl font-semibold uppercase leading-[1.05] tracking-tight text-foreground sm:text-5xl"
      />
      <p className="mt-6 font-serif text-lg leading-relaxed text-muted-foreground">
        An agent that searches the catalog with hybrid retrieval, reads your taste, and calls the recommender. It can only
        recommend titles its tools returned, and you can watch every step it takes.
      </p>
      <div className="mt-10 flex flex-wrap justify-center gap-3">
        <button
          type="button"
          onClick={tryAsGuest}
          disabled={pending}
          className="eyebrow bg-primary px-6 py-3 text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-60"
        >
          {pending ? "Starting" : "Try it as a guest"}
        </button>
        <Link
          href="/login?next=/assistant"
          className="eyebrow border border-border px-6 py-3 text-foreground transition-colors hover:border-primary hover:text-primary"
        >
          Sign in with email
        </Link>
      </div>
      {error && <p className="mt-4 font-mono text-xs text-destructive">{error}</p>}
      <p className="mt-6 font-mono text-[11px] text-muted-foreground">No password. Guests get a smaller daily limit.</p>
    </div>
  );
}

export function AssistantView() {
  const { session, loading } = useAuth();
  const token = session?.access_token;
  const { exchanges, usage, busy, send, stop, reset } = useAssistant(token);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const searchParams = useSearchParams();
  const router = useRouter();

  // A prompt handed over from the landing page fills the composer and waits
  // for the person to send it, so a shared link cannot spend their quota.
  const [draft] = useState(() => searchParams.get("q")?.slice(0, 800) ?? "");
  useEffect(() => {
    if (searchParams.has("q")) router.replace("/assistant");
  }, [searchParams, router]);

  const latest = exchanges.at(-1);
  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [exchanges.length, latest?.picks.length, latest?.message]);

  const traced = useMemo(
    () => exchanges.find((ex) => ex.id === selectedId) ?? latest,
    [exchanges, selectedId, latest],
  );

  if (loading) {
    return <div className="mx-auto min-h-[calc(100dvh-4rem)] max-w-6xl border-x border-border" aria-busy="true" />;
  }

  const guest = session?.user.is_anonymous === true;
  const outOfRuns = usage != null && usage.remaining <= 0;

  return (
    <div className="mx-auto max-w-6xl border-x border-border lg:grid lg:grid-cols-[minmax(0,1fr)_22rem] lg:divide-x lg:divide-border">
      <div className="flex min-h-[calc(100dvh-4rem)] flex-col">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-6 py-4 lg:px-8">
          <div>
            <p className="eyebrow text-primary">Assistant</p>
            <h1 className="mt-1 font-heading text-2xl font-semibold uppercase tracking-tight text-foreground">Ask CineMatch</h1>
          </div>
          {session && (
            <div className="flex items-center gap-4 font-mono text-[11px] text-muted-foreground">
              {usage && (
                <>
                  <span className="flex items-center gap-1.5">
                    <span className={`size-1.5 ${usage.model_available ? "bg-primary" : "bg-amber"}`} />
                    {usage.model_available ? "model online" : "search only"}
                  </span>
                  <span>
                    {usage.remaining}/{usage.limit} left today{guest && " · guest"}
                  </span>
                </>
              )}
              {exchanges.length > 0 && (
                <button type="button" onClick={reset} className="flex items-center gap-1 transition-colors hover:text-primary">
                  <RotateCcw className="size-3" aria-hidden="true" /> New chat
                </button>
              )}
            </div>
          )}
        </div>

        {!session ? (
          <SignedOut />
        ) : (
          <>
            <div className="flex-1">
              {exchanges.length === 0 ? (
                <div className="px-6 py-16 lg:px-8">
                  <SplitHeading
                    as="h2"
                    immediate
                    text="What are you in the mood for?"
                    className="max-w-xl font-heading text-3xl font-semibold uppercase leading-[1.05] tracking-tight text-foreground sm:text-4xl"
                  />
                  <p className="mt-4 max-w-xl font-serif text-lg leading-relaxed text-muted-foreground">
                    Describe a mood, name a title you loved, or ask from your own taste. Every pick comes from the catalog, and
                    the trace shows each step the agent takes.
                  </p>
                  <Suggestions onPick={(prompt) => void send(prompt)} />
                </div>
              ) : (
                exchanges.map((ex) => (
                  <ExchangeView
                    key={ex.id}
                    exchange={ex}
                    token={token}
                    onRetry={(prompt) => void send(prompt)}
                    selected={traced?.id === ex.id}
                    onSelect={() => setSelectedId(ex.id)}
                  />
                ))
              )}
              <div ref={endRef} />
            </div>

            {traced && (
              <details className="border-t border-border lg:hidden">
                <summary className="eyebrow cursor-pointer px-6 py-4 text-primary">Agent trace</summary>
                <AgentTrace exchange={traced} />
              </details>
            )}

            <Composer
              draft={draft}
              busy={busy}
              disabled={outOfRuns}
              disabledReason="Daily limit reached. It resets at midnight UTC."
              onSend={(prompt) => void send(prompt)}
              onStop={stop}
            />
          </>
        )}
      </div>

      <aside className="hidden lg:block" aria-label="Agent trace">
        <div className="sticky top-16 max-h-[calc(100dvh-4rem)] overflow-y-auto" data-lenis-prevent>
          <div className="border-b border-border px-5 py-4">
            <p className="eyebrow text-primary">Agent trace</p>
            <p className="mt-1 font-serif text-sm text-muted-foreground">What the agent did, step by step.</p>
          </div>
          <AgentTrace exchange={traced} />
        </div>
      </aside>
    </div>
  );
}
