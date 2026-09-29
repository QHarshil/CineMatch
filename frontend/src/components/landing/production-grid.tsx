"use client";

import { Activity, FileClock, Gauge, Plug, ShieldCheck, TestTubes } from "lucide-react";
import { SpotlightCard } from "@/components/motion/spotlight-card";

const ITEMS = [
  {
    icon: Activity,
    title: "Streaming agent API",
    body: "POST /assistant streams each tool call as a server-sent event, so the interface shows what the agent intends before the results arrive.",
  },
  {
    icon: ShieldCheck,
    title: "Grounded by construction",
    body: "Picks are checked against tool results on the server. A title no tool returned is dropped and counted in the audit log.",
  },
  {
    icon: Plug,
    title: "Any model provider",
    body: "One OpenAI-compatible client. Ollama on a laptop, a hosted free tier in production, switched by three environment variables.",
  },
  {
    icon: Gauge,
    title: "Budgets that fail closed",
    body: "Each run is counted against per-user and per-network limits before it starts, under a database lock. If that fails, no model call is made.",
  },
  {
    icon: FileClock,
    title: "Audit trail",
    body: "Every run records the model, prompt version, tool calls, and token cost. Prompts are stored only as SHA-256 hashes.",
  },
  {
    icon: TestTubes,
    title: "Evals in the loop",
    body: "Retrieval and agent evals run against the real stack. Unit tests, type checks, and lint run in CI on every push.",
  },
];

export function ProductionGrid() {
  return (
    <div className="grid gap-px border-t border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
      {ITEMS.map(({ icon: Icon, title, body }) => (
        <SpotlightCard key={title} className="bg-background p-6 lg:p-8">
          <Icon className="size-5 text-primary" strokeWidth={1.5} aria-hidden="true" />
          <h3 className="mt-4 font-heading text-lg font-semibold uppercase tracking-wide text-foreground">{title}</h3>
          <p className="mt-2 font-serif leading-relaxed text-muted-foreground">{body}</p>
        </SpotlightCard>
      ))}
    </div>
  );
}
