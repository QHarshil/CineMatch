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
      <div className="w-44 border border-border bg-card px-5 py-5 text-center sm:w-52">
        <p className="font-heading text-lg font-semibold text-foreground sm:text-xl">
          {label}
        </p>
        <p className="eyebrow mt-1 text-primary">{sublabel}</p>
      </div>
      <p className="max-w-[11rem] text-center text-xs leading-relaxed text-muted-foreground">
        {detail}
      </p>
    </div>
  );
}

function Arrow({ delay, revealed }: { delay: string; revealed: boolean }) {
  return (
    <div
      className="mt-5 flex items-center self-start transition-all duration-500 ease-out"
      style={{
        opacity: revealed ? 1 : 0,
        transform: revealed ? "scaleX(1)" : "scaleX(0)",
        transitionDelay: delay,
        transformOrigin: "left",
      }}
    >
      <div className="h-px w-10 bg-primary sm:w-16" />
      <div className="h-0 w-0 border-b-[5px] border-l-[8px] border-t-[5px] border-b-transparent border-l-primary border-t-transparent" />
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
      className="mt-5 flex items-center self-start transition-all duration-500 ease-out"
      style={{
        opacity: revealed ? 0.7 : 0,
        transitionDelay: delay,
      }}
    >
      <span className="whitespace-nowrap font-mono text-[11px] tracking-wide text-muted-foreground">
        {text}
      </span>
    </div>
  );
}

export function PipelineDiagram() {
  const { ref, revealed } = useScrollReveal(0.2);

  return (
    <div ref={ref} className="w-full overflow-x-auto py-8 scrollbar-hide">
      <div className="mx-auto flex w-fit min-w-[700px] items-start gap-3 px-4 sm:gap-4">
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
