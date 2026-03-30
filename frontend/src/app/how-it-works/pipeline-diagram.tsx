"use client";

import { useScrollReveal } from "@/hooks/use-scroll-reveal";

function Stage({
  label,
  sublabel,
  detail,
  delay,
  revealed,
}: {
  label: string;
  sublabel: string;
  detail: string;
  delay: string;
  revealed: boolean;
}) {
  return (
    <div
      className="flex flex-col items-center gap-3 transition-all duration-700 ease-out"
      style={{
        opacity: revealed ? 1 : 0,
        transform: revealed ? "translateY(0)" : "translateY(24px)",
        transitionDelay: delay,
      }}
    >
      <div className="w-44 sm:w-52 border border-border bg-surface px-5 py-5 text-center">
        <p className="font-heading text-lg sm:text-xl font-semibold text-foreground">
          {label}
        </p>
        <p className="mt-1 text-xs tracking-widest uppercase text-gold">
          {sublabel}
        </p>
      </div>
      <p className="text-xs text-muted-foreground max-w-[11rem] text-center leading-relaxed">
        {detail}
      </p>
    </div>
  );
}

function Arrow({ delay, revealed }: { delay: string; revealed: boolean }) {
  return (
    <div
      className="flex items-center self-start mt-5 transition-all duration-500 ease-out"
      style={{
        opacity: revealed ? 1 : 0,
        transform: revealed ? "scaleX(1)" : "scaleX(0)",
        transitionDelay: delay,
        transformOrigin: "left",
      }}
    >
      <div className="w-10 sm:w-16 h-px bg-gold" />
      <div className="w-0 h-0 border-t-[5px] border-t-transparent border-b-[5px] border-b-transparent border-l-[8px] border-l-gold" />
    </div>
  );
}

function DataLabel({
  text,
  delay,
  revealed,
}: {
  text: string;
  delay: string;
  revealed: boolean;
}) {
  return (
    <div
      className="flex items-center self-start mt-5 transition-all duration-500 ease-out"
      style={{
        opacity: revealed ? 0.7 : 0,
        transitionDelay: delay,
      }}
    >
      <span className="text-[11px] text-muted-foreground tracking-wide whitespace-nowrap">
        {text}
      </span>
    </div>
  );
}

export function PipelineDiagram() {
  const { ref, revealed } = useScrollReveal(0.2);

  return (
    <div ref={ref} className="w-full overflow-x-auto py-8 scrollbar-hide">
      <div className="flex items-start justify-center gap-3 sm:gap-4 min-w-[700px] px-4">
        <Stage
          label="User Profile"
          sublabel="Embedding"
          detail="Interaction history encoded as a 1536-dim vector"
          delay="0ms"
          revealed={revealed}
        />
        <Arrow delay="200ms" revealed={revealed} />
        <Stage
          label="Retrieval"
          sublabel="pgvector kNN"
          detail="Cosine similarity search finds the 50 closest movies"
          delay="300ms"
          revealed={revealed}
        />
        <DataLabel text="50 candidates" delay="450ms" revealed={revealed} />
        <Arrow delay="500ms" revealed={revealed} />
        <Stage
          label="Ranking"
          sublabel="ML Scoring"
          detail="Multi-feature model re-scores and sorts the candidates"
          delay="600ms"
          revealed={revealed}
        />
        <DataLabel text="Top 20" delay="750ms" revealed={revealed} />
        <Arrow delay="800ms" revealed={revealed} />
        <Stage
          label="Results"
          sublabel="Personalized"
          detail="Your top recommendations, ordered by predicted relevance"
          delay="900ms"
          revealed={revealed}
        />
      </div>
    </div>
  );
}
