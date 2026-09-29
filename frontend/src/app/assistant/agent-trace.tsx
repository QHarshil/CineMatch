"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import type { Exchange } from "@/lib/assistant-session";
import { TOOLS, toolLabel } from "@/lib/assistant-session";

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      onClick={() => {
        void navigator.clipboard?.writeText(value).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
      aria-label="Copy run ID"
      className="text-muted-foreground transition-colors hover:text-primary"
    >
      {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
    </button>
  );
}

/**
 * The full record of one run: every tool call with its arguments and results,
 * the grounding check, and cost. Mirrors what the backend writes to the
 * assistant_runs audit table.
 */
export function AgentTrace({ exchange }: { exchange?: Exchange }) {
  if (!exchange) {
    return (
      <div className="px-5 py-6">
        <p className="font-serif text-sm leading-relaxed text-muted-foreground">
          Each request runs a tool-calling loop. The trace for your latest request appears here: what the agent
          searched, what came back, and what it cost.
        </p>
        <ul className="mt-6 space-y-4">
          {Object.entries(TOOLS).map(([name, tool]) => (
            <li key={name}>
              <p className="font-mono text-xs text-primary">{name}</p>
              <p className="mt-0.5 font-serif text-sm text-muted-foreground">{tool.description}</p>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  const run = exchange.run;
  return (
    <div className="px-5 py-5">
      <p className="font-mono text-[11px] text-muted-foreground">
        {exchange.model ?? "model pending"}
        {exchange.promptVersion && ` · ${exchange.promptVersion}`}
      </p>

      <ol className="mt-4 space-y-4 border-l border-border pl-4">
        {exchange.steps.map((step, i) => (
          <li key={step.id} className="relative duration-300 animate-in fade-in">
            <span className="absolute -left-[21px] top-1 size-2.5 border border-primary bg-background" />
            <p className="eyebrow text-primary">
              {String(i + 1).padStart(2, "0")} {toolLabel(step.tool)}
            </p>
            <pre className="mt-1.5 whitespace-pre-wrap break-words bg-wash px-2.5 py-2 font-mono text-[11px] leading-relaxed text-foreground">
              {step.tool}({JSON.stringify(step.args, null, 1).replace(/\n\s*/g, " ")})
            </pre>
            <p className="mt-1.5 font-mono text-[11px] text-muted-foreground">
              {step.status === "running"
                ? "running"
                : step.error
                  ? step.error
                  : `${step.count ?? 0} results · ${step.latencyMs ?? 0} ms${step.retrieval ? ` · ${step.retrieval}` : ""}`}
            </p>
            {step.titles && step.titles.length > 0 && (
              <p className="mt-1 font-serif text-xs italic text-muted-foreground">{step.titles.join(", ")}</p>
            )}
          </li>
        ))}
        {(exchange.picks.length > 0 || run) && (
          <li className="relative">
            <span className="absolute -left-[21px] top-1 size-2.5 bg-primary" />
            <p className="eyebrow text-primary">{String(exchange.steps.length + 1).padStart(2, "0")} Grounding check</p>
            <p className="mt-1.5 font-mono text-[11px] text-muted-foreground">
              {exchange.picks.length} picks verified against tool results
              {exchange.dropped > 0 ? `, ${exchange.dropped} rejected` : ", none rejected"}
            </p>
          </li>
        )}
      </ol>

      {run && (
        <dl className="mt-6 grid grid-cols-2 gap-px border border-border bg-border font-mono text-[11px]">
          {[
            ["Status", run.status],
            ["Latency", `${(run.latency_ms / 1000).toFixed(1)} s`],
            ["Tokens in", run.usage.input_tokens.toLocaleString()],
            ["Tokens out", run.usage.output_tokens.toLocaleString()],
          ].map(([label, value]) => (
            <div key={label} className="bg-background px-3 py-2">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="mt-0.5 text-foreground">{value}</dd>
            </div>
          ))}
          <div className="col-span-2 flex items-center justify-between gap-2 bg-background px-3 py-2">
            <div className="min-w-0">
              <dt className="text-muted-foreground">Audit run ID</dt>
              <dd className="mt-0.5 truncate text-foreground">{run.run_id}</dd>
            </div>
            <CopyButton value={run.run_id} />
          </div>
        </dl>
      )}

      <p className="mt-6 font-serif text-xs leading-relaxed text-muted-foreground">
        Tools are read-only. Your profile changes only when you like or dislike a pick.
      </p>
    </div>
  );
}
