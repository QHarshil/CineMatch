"use client";

import { AlertCircle, Check, Loader2, RotateCcw, ShieldCheck } from "lucide-react";
import type { Exchange, ToolStep } from "@/lib/assistant-session";
import { TOOL_NAMES } from "@/lib/assistant-session";
import { PickCard } from "./pick-card";

function StepIcon({ status }: { status: ToolStep["status"] }) {
  if (status === "running") return <Loader2 className="size-3.5 animate-spin text-primary" aria-hidden="true" />;
  if (status === "error") return <AlertCircle className="size-3.5 text-destructive" aria-hidden="true" />;
  return <Check className="size-3.5 text-primary" aria-hidden="true" />;
}

/** The agent's tool calls as they happen: an intent preview, then the result. */
export function StepList({ steps }: { steps: ToolStep[] }) {
  return (
    <ol className="space-y-1.5">
      {steps.map((step) => (
        <li key={step.id} className="flex items-start gap-2.5 duration-300 animate-in fade-in slide-in-from-left-2">
          <span className="mt-0.5 shrink-0">
            <StepIcon status={step.status} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="font-mono text-xs text-foreground">{step.label}</span>
            {step.status !== "running" && (
              <span className="ml-2 font-mono text-[11px] text-muted-foreground">
                {step.error ? step.error : `${step.count ?? 0} results · ${step.latencyMs ?? 0} ms`}
              </span>
            )}
          </span>
        </li>
      ))}
    </ol>
  );
}

function formatReset(resetsAt?: string): string | null {
  if (!resetsAt) return null;
  const date = new Date(resetsAt);
  return Number.isNaN(date.getTime()) ? null : date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

interface ExchangeViewProps {
  exchange: Exchange;
  token?: string;
  onRetry?: (prompt: string) => void;
  selected?: boolean;
  onSelect?: () => void;
}

export function ExchangeView({ exchange, token, onRetry, selected, onSelect }: ExchangeViewProps) {
  const streaming = exchange.status === "streaming";
  const thinking = streaming && exchange.steps.length === 0 && !exchange.message;
  const resetTime = formatReset(exchange.error?.resetsAt);

  return (
    <section className="border-b border-border" aria-busy={streaming}>
      <div className="px-6 pb-4 pt-8 lg:px-8">
        <p className="eyebrow text-muted-foreground">You</p>
        <p className="mt-2 font-heading text-xl font-medium leading-snug text-foreground sm:text-2xl">{exchange.prompt}</p>
      </div>

      <div className="px-6 pb-8 lg:px-8">
        <button
          type="button"
          onClick={onSelect}
          className={`eyebrow mb-3 flex items-center gap-2 transition-colors ${selected ? "text-primary" : "text-muted-foreground hover:text-primary"}`}
        >
          <span className={`size-1.5 ${streaming ? "animate-pulse bg-primary" : "bg-primary/60"}`} />
          CineMatch
          {exchange.model && <span className="normal-case tracking-normal text-muted-foreground/80">· {exchange.model}</span>}
        </button>

        <div aria-live="polite" className="space-y-4">
          {thinking && <p className="font-mono text-xs text-muted-foreground animate-pulse">Reading your request</p>}
          {exchange.steps.length > 0 && <StepList steps={exchange.steps} />}

          {exchange.message && (
            <p className="max-w-2xl font-serif text-lg leading-relaxed text-foreground duration-500 animate-in fade-in">{exchange.message}</p>
          )}

          {exchange.status === "fallback" && (
            <p className="eyebrow inline-flex items-center gap-2 border border-amber/60 bg-amber/10 px-2.5 py-1 text-[#8a5a00]">
              Search results, not the model
            </p>
          )}
        </div>

        {exchange.picks.length > 0 && (
          <div className="mt-6 grid gap-px border border-border bg-border sm:grid-cols-2">
            {exchange.picks.map((pick, i) => (
              <PickCard key={pick.movie.id} pick={pick} token={token} index={i} />
            ))}
            {exchange.picks.length % 2 === 1 && <div aria-hidden="true" className="hidden bg-background sm:block" />}
          </div>
        )}

        {exchange.error && (exchange.status === "failed" || exchange.status === "error") && (
          <div role="alert" className="mt-4 flex flex-wrap items-center gap-3 border border-destructive/40 bg-destructive/5 px-4 py-3">
            <AlertCircle className="size-4 text-destructive" aria-hidden="true" />
            <p className="font-mono text-xs text-destructive">
              {exchange.error.message}
              {resetTime && ` Resets at ${resetTime}.`}
            </p>
            {onRetry && exchange.error.status !== 429 && (
              <button
                type="button"
                onClick={() => onRetry(exchange.prompt)}
                className="eyebrow ml-auto flex items-center gap-1.5 text-foreground transition-colors hover:text-primary"
              >
                <RotateCcw className="size-3" aria-hidden="true" /> Retry
              </button>
            )}
          </div>
        )}

        {exchange.run && (
          <p className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] text-muted-foreground">
            <span className="flex items-center gap-1">
              <ShieldCheck className="size-3 text-primary" aria-hidden="true" />
              {exchange.picks.length > 0
                ? `${exchange.picks.length} grounded ${exchange.picks.length === 1 ? "pick" : "picks"}`
                : "no picks"}
              {exchange.dropped > 0 && `, ${exchange.dropped} ungrounded blocked`}
            </span>
            <span>{(exchange.run.latency_ms / 1000).toFixed(1)} s</span>
            {exchange.run.usage.input_tokens > 0 && (
              <span>{(exchange.run.usage.input_tokens + exchange.run.usage.output_tokens).toLocaleString()} tokens</span>
            )}
            <span>
              {exchange.steps.length} tool {exchange.steps.length === 1 ? "call" : "calls"}
              {exchange.steps.length > 0 && `: ${[...new Set(exchange.steps.map((s) => TOOL_NAMES[s.tool] ?? s.tool))].join(", ")}`}
            </span>
          </p>
        )}
      </div>
    </section>
  );
}
