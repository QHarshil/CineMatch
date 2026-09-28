"use client";

import { CountUp } from "@/components/motion/count-up";

export interface Metric {
  value: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
  label: string;
  detail: string;
}

/**
 * Measured results only. Each number comes from a script in eval/ that
 * anyone can rerun; the How it works page shows the method.
 */
export function MetricsBand({ metrics }: { metrics: Metric[] }) {
  return (
    <div className="grid gap-px border-t border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
      {metrics.map((m) => (
        <div key={m.label} className="bg-background p-6 lg:p-8">
          <CountUp
            value={m.value}
            decimals={m.decimals}
            prefix={m.prefix}
            suffix={m.suffix}
            className="font-heading text-5xl font-semibold tracking-tight text-foreground tabular-nums"
          />
          <p className="eyebrow mt-3 text-primary">{m.label}</p>
          <p className="mt-2 font-serif text-sm leading-relaxed text-muted-foreground">{m.detail}</p>
        </div>
      ))}
    </div>
  );
}
