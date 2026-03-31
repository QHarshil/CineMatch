"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/lib/auth-context";
import { toggleInteraction, fetchInteractionState, submitRating, RateLimitError } from "@/lib/api";
import { useToast } from "@/components/toast";
import { Heart, ThumbsDown, Eye, Bookmark, Star } from "lucide-react";
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

const COOLDOWN_MS = 500;

export function InteractionButtons({ movieId }: { movieId: string }) {
  const { session } = useAuth();
  const { showToast } = useToast();
  const [activeTypes, setActiveTypes] = useState<Set<InteractionType>>(new Set());
  const [rating, setRating] = useState<number>(0);
  const [hoverStar, setHoverStar] = useState<number>(0);
  const [submitting, setSubmitting] = useState(false);
  const [cooldown, setCooldown] = useState(false);
  const [showAuthHint, setShowAuthHint] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const loadState = useCallback(async () => {
    if (!session) return;
    try {
      const state = await fetchInteractionState(session.access_token, movieId);
      setActiveTypes(new Set(state.interactions as InteractionType[]));
      setRating(state.rating ?? 0);
    } catch {
      // Silently fail on load — the user can still interact
    } finally {
      setLoaded(true);
    }
  }, [session, movieId]);

  useEffect(() => {
    loadState();
  }, [loadState]);

  async function handleToggle(type: InteractionType) {
    if (!session) {
      setShowAuthHint(true);
      return;
    }
    if (cooldown) return;

    setSubmitting(true);
    setCooldown(true);
    try {
      const resp = await toggleInteraction(session.access_token, movieId, type);

      setActiveTypes((prev) => {
        const next = new Set(prev);
        if (resp.action === "added") {
          next.add(type);
          // Mirror backend mutual exclusivity in UI immediately
          if (type === "like") next.delete("dislike");
          if (type === "dislike") next.delete("like");
        } else {
          next.delete(type);
        }
        return next;
      });
    } catch (err) {
      if (err instanceof RateLimitError) {
        showToast("Slow down, try again in a moment");
      } else {
        showToast("Failed to save, please try again");
      }
    } finally {
      setSubmitting(false);
      setTimeout(() => setCooldown(false), COOLDOWN_MS);
    }
  }

  async function handleRating(score: number) {
    if (!session) {
      setShowAuthHint(true);
      return;
    }

    // Clicking the same star clears the rating
    const newScore = score === rating ? 0 : score;

    try {
      await submitRating(session.access_token, movieId, newScore);
      setRating(newScore);
    } catch (err) {
      if (err instanceof RateLimitError) {
        showToast("Slow down, try again in a moment");
      } else {
        showToast("Failed to save rating");
      }
    }
  }

  return (
    <div className="space-y-4">
      {/* Interaction buttons */}
      <div className="flex items-center gap-6">
        {INTERACTIONS.map(({ type, label, icon: Icon }) => {
          const isActive = activeTypes.has(type);
          return (
            <button
              key={type}
              disabled={submitting || cooldown}
              onClick={() => handleToggle(type)}
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

      {/* Star rating */}
      {session && loaded && (
        <div className="flex items-center gap-1">
          <span className="text-xs text-muted-foreground mr-2">Your rating</span>
          {Array.from({ length: 10 }, (_, i) => i + 1).map((star) => {
            const filled = star <= (hoverStar || rating);
            return (
              <button
                key={star}
                onClick={() => handleRating(star)}
                onMouseEnter={() => setHoverStar(star)}
                onMouseLeave={() => setHoverStar(0)}
                className="transition-colors duration-150"
              >
                <Star
                  className={`w-5 h-5 ${
                    filled
                      ? "text-gold fill-gold"
                      : "text-muted-foreground/40 hover:text-gold/60"
                  }`}
                  strokeWidth={1.5}
                />
              </button>
            );
          })}
          {rating > 0 && (
            <span className="text-xs text-gold ml-2 font-medium">{rating}/10</span>
          )}
        </div>
      )}

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
