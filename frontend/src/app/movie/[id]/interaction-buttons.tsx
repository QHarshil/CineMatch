"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { recordInteraction } from "@/lib/api";
import { Button } from "@/components/ui/button";
import type { InteractionType } from "@/types/movie";

const INTERACTION_LABELS: { type: InteractionType; label: string }[] = [
  { type: "like", label: "Like" },
  { type: "dislike", label: "Dislike" },
  { type: "watch", label: "Watched" },
  { type: "skip", label: "Skip" },
];

export function InteractionButtons({ movieId }: { movieId: string }) {
  const { session } = useAuth();
  const [selected, setSelected] = useState<InteractionType | null>(null);
  const [submitting, setSubmitting] = useState(false);

  if (!session) {
    return (
      <p className="text-sm text-muted-foreground">
        Sign in to rate this movie.
      </p>
    );
  }

  async function handleClick(type: InteractionType) {
    if (!session) return;
    setSubmitting(true);
    try {
      await recordInteraction(session.access_token, movieId, type);
      setSelected(type);
    } catch (err) {
      console.error("Failed to record interaction:", err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex flex-wrap gap-2">
      {INTERACTION_LABELS.map(({ type, label }) => (
        <Button
          key={type}
          variant={selected === type ? "default" : "outline"}
          size="sm"
          disabled={submitting}
          onClick={() => handleClick(type)}
        >
          {label}
        </Button>
      ))}
    </div>
  );
}
