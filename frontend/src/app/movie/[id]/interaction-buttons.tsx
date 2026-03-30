"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { recordInteraction } from "@/lib/api";
import { Heart, ThumbsDown, Eye, Bookmark } from "lucide-react";
import Link from "next/link";
import type { InteractionType } from "@/types/movie";

const INTERACTIONS: {
  type: InteractionType;
  label: string;
  icon: typeof Heart;
}[] = [
  { type: "like", label: "Like", icon: Heart },
  { type: "dislike", label: "Dislike", icon: ThumbsDown },
  { type: "watch", label: "Watched", icon: Eye },
  { type: "skip", label: "Watchlist", icon: Bookmark },
];

export function InteractionButtons({ movieId }: { movieId: string }) {
  const { session } = useAuth();
  const [selected, setSelected] = useState<InteractionType | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [showAuthHint, setShowAuthHint] = useState(false);

  async function handleClick(type: InteractionType) {
    if (!session) {
      setShowAuthHint(true);
      return;
    }
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
    <div className="space-y-3">
      <div className="flex items-center gap-6">
        {INTERACTIONS.map(({ type, label, icon: Icon }) => {
          const isActive = selected === type;
          return (
            <button
              key={type}
              disabled={submitting}
              onClick={() => handleClick(type)}
              className={`flex flex-col items-center gap-1.5 transition-colors duration-200 disabled:opacity-50 group ${
                isActive ? "text-gold" : "text-muted-foreground"
              }`}
            >
              <div
                className={`w-10 h-10 flex items-center justify-center rounded-full transition-colors duration-200 ${
                  isActive
                    ? "bg-gold/15"
                    : "bg-surface group-hover:bg-surface-hover"
                }`}
              >
                <Icon
                  className="w-4.5 h-4.5"
                  strokeWidth={1.5}
                  fill={isActive && (type === "like" || type === "skip") ? "currentColor" : "none"}
                />
              </div>
              <span className="text-[11px]">{label}</span>
            </button>
          );
        })}
      </div>

      {showAuthHint && !session && (
        <p className="text-xs text-muted-foreground">
          <Link href="/login" className="text-gold hover:text-gold-dim transition-colors">
            Sign in
          </Link>{" "}
          to save your preferences and get personalized recommendations.
        </p>
      )}
    </div>
  );
}
