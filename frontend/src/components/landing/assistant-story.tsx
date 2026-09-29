"use client";

import { useRef, useState, type ReactNode } from "react";
import { Check, X } from "lucide-react";
import { MOTION_OK, ScrollTrigger, gsap, useGSAP } from "@/lib/motion";

interface Step {
  n: string;
  title: string;
  body: string;
  visual: ReactNode;
}

function Panel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="border border-primary/40 bg-[#f3f7ff] ring-1 ring-inset ring-primary/10">
      <div className="flex items-center gap-1.5 border-b border-primary/25 bg-white/60 px-4 py-2.5">
        <span className="size-2 rounded-full bg-primary/70" />
        <span className="size-2 rounded-full bg-primary/40" />
        <span className="size-2 rounded-full bg-primary/20" />
        <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.18em] text-primary/70">{label}</span>
      </div>
      <div className="p-5 font-mono text-[13px] leading-6 text-foreground">{children}</div>
    </div>
  );
}

const RANKERS = [
  { name: "semantic", width: [92, 78, 64, 40] },
  { name: "full text", width: [30, 70, 0, 55] },
  { name: "title", width: [0, 0, 88, 0] },
];

const STEPS: Step[] = [
  {
    n: "01",
    title: "Plan",
    body: "The model reads the request and chooses tools. Constraints like a series or under two hours become filters, so every pick meets them.",
    visual: (
      <Panel label="tool_call">
        <p className="text-muted-foreground">&quot;a Korean thriller series, not too long&quot;</p>
        <p className="mt-3">
          <span className="text-primary">search_catalog</span>({"{"}
        </p>
        <p className="pl-4">
          query: <span className="text-[#0f766e]">&quot;tense thriller&quot;</span>,
        </p>
        <p className="pl-4">
          media_type: <span className="text-[#0f766e]">&quot;tv&quot;</span>,
        </p>
        <p className="pl-4">
          language: <span className="text-[#0f766e]">&quot;ko&quot;</span>,
        </p>
        <p className="pl-4">
          max_runtime: <span className="text-[#9a6308]">60</span>
        </p>
        <p>{"})"}</p>
      </Panel>
    ),
  },
  {
    n: "02",
    title: "Retrieve",
    body: "Hybrid search runs three rankers over the catalog: embeddings for meaning, full text for words, trigrams for typos. Reciprocal rank fusion merges them by rank, so no scores need calibrating.",
    visual: (
      <Panel label="search_titles_hybrid">
        <div className="space-y-3">
          {RANKERS.map((r) => (
            <div key={r.name}>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground">{r.name}</p>
              <div className="mt-1 grid grid-cols-4 gap-1">
                {r.width.map((w, i) => (
                  <div key={i} className="h-2 bg-border">
                    <div data-bar className="h-full bg-primary" style={{ width: `${w}%` }} />
                  </div>
                ))}
              </div>
            </div>
          ))}
          <p className="border-t border-primary/20 pt-3 text-[11px] uppercase tracking-wider text-primary">
            fused: score = Σ 1 / (60 + rank)
          </p>
        </div>
      </Panel>
    ),
  },
  {
    n: "03",
    title: "Ground",
    body: "Every title a tool returns gets a short ref. The final answer may only cite refs the agent has seen. An invented title is rejected on the server before it reaches you, and the rejection is logged.",
    visual: (
      <Panel label="present_picks">
        {[
          { ref: "t1", title: "Mousetrap", ok: true },
          { ref: "t4", title: "Bloodhounds", ok: true },
          { ref: "t7", title: "Squid Game", ok: true },
          { ref: "t19", title: "a title no tool returned", ok: false },
        ].map((row) => (
          <p key={row.ref} className={`flex items-center gap-3 ${row.ok ? "" : "text-muted-foreground line-through"}`}>
            {row.ok ? <Check className="size-3.5 text-primary" /> : <X className="size-3.5 text-destructive" />}
            <span className="w-8 text-primary">{row.ref}</span>
            {row.title}
          </p>
        ))}
        <p className="mt-3 border-t border-primary/20 pt-3 text-[11px] uppercase tracking-wider text-primary">
          3 grounded · 1 rejected
        </p>
      </Panel>
    ),
  },
  {
    n: "04",
    title: "Rank and explain",
    body: "Your personal feed runs the two-stage recommender: pgvector retrieves 50 candidates near your taste, then a LightGBM LambdaMART model re-orders them. Its SHAP values say why each title ranked where it did.",
    visual: (
      <Panel label="lambdamart-v1">
        <p className="text-muted-foreground">why this pick</p>
        {[
          { name: "close to your taste", w: 88 },
          { name: "highly rated", w: 46 },
          { name: "recent release", w: 22 },
        ].map((f) => (
          <div key={f.name} className="mt-3">
            <div className="flex justify-between text-[11px]">
              <span>{f.name}</span>
              <span className="text-muted-foreground">+{(f.w / 400).toFixed(2)}</span>
            </div>
            <div className="mt-1 h-2 bg-border">
              <div data-bar className="h-full bg-primary" style={{ width: `${f.w}%` }} />
            </div>
          </div>
        ))}
        <p className="mt-4 border-t border-primary/20 pt-3 text-[11px] uppercase tracking-wider text-primary">
          because you liked Arrival
        </p>
      </Panel>
    ),
  },
  {
    n: "05",
    title: "Audit and budget",
    body: "Each run is logged with its tools, token cost, and a hashed prompt. Per-user limits and a global token budget are checked before any model call. Past the budget, the assistant answers from search alone.",
    visual: (
      <Panel label="assistant_runs">
        <p>
          status <span className="text-primary">picks</span> · prompt{" "}
          <span className="text-muted-foreground">sha256:8afe…</span>
        </p>
        <p>tools search_catalog, find_similar</p>
        <p>tokens 6,213 · 11.4 s · ungrounded 0</p>
        <div className="mt-4">
          <div className="flex justify-between text-[11px] text-muted-foreground">
            <span>today&apos;s budget</span>
            <span>31%</span>
          </div>
          <div className="mt-1 h-2 bg-border">
            <div data-bar className="h-full bg-primary" style={{ width: "31%" }} />
          </div>
        </div>
      </Panel>
    ),
  },
];

/**
 * A scroll story of one assistant run. On wide screens the visual pins beside
 * the steps and changes as each step becomes active; on small screens each
 * visual sits under its step.
 */
export function AssistantStory() {
  const ref = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);

  useGSAP(
    () => {
      const el = ref.current;
      if (!el) return;
      const steps = gsap.utils.toArray<HTMLElement>("[data-step]", el);
      const triggers = steps.map((step, i) =>
        ScrollTrigger.create({
          trigger: step,
          start: "top 55%",
          end: "bottom 55%",
          onToggle: (self) => {
            if (self.isActive) setActive(i);
          },
        }),
      );
      const mm = gsap.matchMedia();
      mm.add(MOTION_OK, () => {
        gsap.utils.toArray<HTMLElement>("[data-visual]", el).forEach((visual) => {
          const bars = visual.querySelectorAll("[data-bar]");
          if (bars.length === 0) return;
          gsap.from(bars, {
            scaleX: 0,
            transformOrigin: "left center",
            duration: 0.9,
            ease: "power2.out",
            stagger: 0.05,
            scrollTrigger: { trigger: visual, start: "top 80%", once: true },
          });
        });
      });
      return () => {
        triggers.forEach((t) => t.kill());
        mm.revert();
      };
    },
    { scope: ref },
  );

  return (
    <div ref={ref} className="grid lg:grid-cols-2">
      <ol className="border-border lg:border-r">
        {STEPS.map((step, i) => (
          <li
            key={step.n}
            data-step
            className={`border-b border-border px-6 py-12 transition-colors duration-500 last:border-b-0 lg:flex lg:min-h-[62vh] lg:flex-col lg:justify-center lg:px-8 ${
              active === i ? "lg:bg-wash/60" : ""
            }`}
          >
            <p
              className={`eyebrow transition-colors duration-500 ${active === i ? "text-primary" : "text-muted-foreground"}`}
            >
              {step.n} · {step.title}
            </p>
            <p className="mt-4 max-w-md font-serif text-xl leading-relaxed text-foreground">{step.body}</p>
            <div data-visual className="mt-8 lg:hidden">
              {step.visual}
            </div>
          </li>
        ))}
      </ol>
      <div className="relative hidden lg:block">
        <div className="sticky top-24 p-8">
          <div className="relative min-h-[360px]">
            {STEPS.map((step, i) => (
              <div
                key={step.n}
                data-visual
                aria-hidden={active !== i}
                className={`absolute inset-x-0 top-0 transition-all duration-700 ease-out ${
                  active === i ? "translate-y-0 opacity-100" : "pointer-events-none translate-y-4 opacity-0"
                }`}
              >
                {step.visual}
              </div>
            ))}
          </div>
          <div className="mt-6 flex gap-1.5" aria-hidden="true">
            {STEPS.map((step, i) => (
              <span
                key={step.n}
                className={`h-0.5 flex-1 transition-colors duration-500 ${i <= active ? "bg-primary" : "bg-border"}`}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
