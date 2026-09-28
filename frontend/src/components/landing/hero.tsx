"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { SplitHeading } from "@/components/motion/split-heading";
import { ScrambleCycle } from "@/components/motion/scramble-cycle";
import { VantaNet } from "@/components/motion/vanta-net";

const EXAMPLES = [
  "a slow-burn sci-fi that makes me think",
  "something like Parasite, but a series",
  "a Korean thriller under two hours",
  "a cozy animated film for a rainy night",
];

const QUICK_PROMPTS = ["Movies like Prisoners", "Horror from 2022 on", "Based on what I like"];

export function LandingHero({ catalogSize }: { catalogSize: number }) {
  const router = useRouter();
  const [prompt, setPrompt] = useState("");

  function ask(text: string) {
    const q = text.trim();
    if (q) router.push(`/assistant?q=${encodeURIComponent(q)}`);
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    ask(prompt);
  }

  const size = catalogSize > 0 ? catalogSize.toLocaleString("en-US") : "1,800+";

  return (
    <section className="relative overflow-hidden">
      <VantaNet className="absolute inset-0" />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.92)_0%,rgba(255,255,255,0.7)_45%,rgba(255,255,255,0)_75%)]"
      />
      <div aria-hidden="true" className="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-gradient-to-b from-transparent to-background" />

      <div className="relative z-10 px-6 pb-24 pt-32 text-center lg:pt-40">
        <p className="eyebrow text-muted-foreground">
          Applied AI
          <span className="mx-2 text-primary">/</span>
          hybrid retrieval
          <span className="mx-2 text-primary">/</span>
          grounded agent
        </p>

        <SplitHeading
          as="h1"
          immediate
          text="Describe a mood. Get a film that exists."
          className="mx-auto mt-6 max-w-4xl font-heading text-4xl font-semibold uppercase leading-[1.02] tracking-tight text-foreground sm:text-6xl lg:text-7xl"
        />

        <p className="mx-auto mt-6 max-w-2xl font-serif text-lg leading-relaxed text-muted-foreground">
          CineMatch pairs a two-stage recommender with a grounded AI assistant. It searches {size} films and series by
          meaning, re-ranks with LambdaMART, and can only recommend titles it retrieved.
        </p>

        <form onSubmit={submit} className="mx-auto mt-10 max-w-2xl text-left">
          <label htmlFor="hero-prompt" className="sr-only">
            Describe what you want to watch
          </label>
          <div className="flex items-stretch border border-primary/50 bg-background shadow-[0_18px_50px_-20px_rgba(47,84,255,0.35)] transition-colors focus-within:border-primary">
            <div className="relative flex-1">
              <input
                id="hero-prompt"
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                autoComplete="off"
                className="h-14 w-full bg-transparent px-5 font-serif text-base text-foreground focus:outline-none sm:text-lg"
              />
              {prompt === "" && (
                <span
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-y-0 left-5 right-3 flex items-center overflow-hidden whitespace-nowrap font-serif text-base text-muted-foreground sm:text-lg"
                >
                  <ScrambleCycle phrases={EXAMPLES} />
                </span>
              )}
            </div>
            <button
              type="submit"
              className="eyebrow flex items-center gap-2 bg-primary px-5 text-primary-foreground transition-colors hover:bg-primary/90 sm:px-7"
            >
              Ask <ArrowRight className="size-3.5" aria-hidden="true" />
            </button>
          </div>
        </form>

        <div className="mx-auto mt-4 flex max-w-2xl flex-wrap justify-center gap-2">
          {QUICK_PROMPTS.map((q) => (
            <button
              key={q}
              type="button"
              onClick={() => ask(q)}
              className="border border-border bg-background/80 px-3 py-1.5 font-mono text-xs text-muted-foreground backdrop-blur transition-colors hover:border-primary hover:text-primary"
            >
              {q}
            </button>
          ))}
        </div>

        <p className="mt-10 font-mono text-[11px] text-muted-foreground">
          No account needed to try it.{" "}
          <Link href="/how-it-works" className="text-primary underline-offset-4 hover:underline">
            See how it works
          </Link>
        </p>
      </div>
    </section>
  );
}
