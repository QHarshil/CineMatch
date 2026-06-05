"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { TypingText } from "@/components/typing-text";

// The pipeline the engine actually runs, shown as the editorial "setup steps".
const STEPS = [
  {
    n: "01",
    name: "Profile",
    stage: "your taste",
    command: "taste = embed(your_likes)",
    delay: 250,
  },
  {
    n: "02",
    name: "Retrieve",
    stage: "pgvector kNN",
    command: "candidates = pgvector.knn(taste, k=50)",
    delay: 1600,
  },
  {
    n: "03",
    name: "Re-rank",
    stage: "lambdaMART",
    command: "picks = lambdaMART.rank(candidates)",
    delay: 3500,
  },
];

export function LandingHero() {
  const { user, loading } = useAuth();

  const primaryCta =
    !loading && user
      ? { href: "/for-you", label: "Open your picks" }
      : { href: "/login", label: "Start matching" };

  return (
    <section className="px-6 pb-16 pt-28 text-center lg:pt-32">
      <div className="mx-auto max-w-3xl">
        <p className="eyebrow text-muted-foreground">
          Two-stage engine
          <span className="mx-2 text-primary">/</span>
          pgvector + LambdaMART
        </p>

        <h1 className="mt-6 font-heading text-3xl font-semibold uppercase leading-[1.06] tracking-tight text-foreground sm:text-5xl sm:leading-[1.02] lg:text-7xl">
          Recommendations that learn{" "}
          <span className="text-primary">your taste.</span>
        </h1>

        <p className="mx-auto mt-6 max-w-2xl font-serif text-lg leading-relaxed text-muted-foreground">
          Not another row of what is trending. CineMatch builds a profile from
          what you watch, retrieves the closest titles with a pgvector index,
          and re-ranks them with a learned model so the next pick fits you, not
          the crowd.
        </p>
      </div>

      {/* The pipeline, typed out as setup steps */}
      <div className="mx-auto mt-12 max-w-2xl space-y-4 text-left">
        {STEPS.map((step) => (
          <div key={step.n}>
            <div className="eyebrow mb-2 flex items-center justify-between text-muted-foreground">
              <span>
                {step.n}. {step.name}
              </span>
              <span className="text-primary/60">{step.stage}</span>
            </div>
            <div className="border border-border bg-wash px-4 py-3 font-mono text-sm text-foreground">
              <span className="select-none text-primary">$ </span>
              <TypingText
                text={step.command}
                speed={42}
                startDelay={step.delay}
                startOnVisible={false}
              />
            </div>
          </div>
        ))}
      </div>

      <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
        <Link
          href={primaryCta.href}
          className="eyebrow bg-primary px-6 py-3 text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {primaryCta.label}
        </Link>
        <Link
          href="/how-it-works"
          className="eyebrow border border-border px-6 py-3 text-foreground transition-colors hover:border-primary hover:text-primary"
        >
          See how it works
        </Link>
      </div>
    </section>
  );
}
