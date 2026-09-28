"use client";

import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { Heart, Star, ThumbsDown } from "lucide-react";
import { RateLimitError, toggleInteraction } from "@/lib/api";
import { confidenceOf, TOOL_NAMES } from "@/lib/assistant-session";
import type { AssistantPick } from "@/lib/assistant-stream";
import { useToast } from "@/components/toast";

const POSTER_BASE = "https://image.tmdb.org/t/p/w185";

type Feedback = "like" | "dislike" | null;

/**
 * One recommended title. The reason comes from the model; the confidence
 * meter comes from retrieval similarity. Like and dislike are the only way a
 * run changes the profile, and only on a click.
 */
export function PickCard({ pick, token, index }: { pick: AssistantPick; token?: string; index: number }) {
  const { movie } = pick;
  const { showToast } = useToast();
  const [feedback, setFeedback] = useState<Feedback>(null);
  const [pending, setPending] = useState(false);
  const confidence = confidenceOf(pick.similarity);
  const kind = movie.media_type === "tv" ? "Series" : "Film";

  async function give(type: "like" | "dislike") {
    if (!token || pending) return;
    setPending(true);
    try {
      const result = await toggleInteraction(token, movie.id, type);
      const next = result.action === "added" ? type : null;
      setFeedback(next);
      if (next === "like") showToast(`Added ${movie.title} to your taste`, "info");
      if (next === "dislike") showToast(`You will see less like ${movie.title}`, "info");
    } catch (err) {
      showToast(err instanceof RateLimitError ? "Slow down a moment, then try again" : "Could not save that", "error");
    } finally {
      setPending(false);
    }
  }

  return (
    <article
      className="group flex gap-4 bg-background p-4 duration-500 animate-in fade-in slide-in-from-bottom-2 fill-mode-both"
      style={{ animationDelay: `${index * 70}ms` }}
    >
      <Link
        href={`/movie/${movie.id}`}
        className="relative aspect-[2/3] w-20 shrink-0 overflow-hidden border border-border bg-muted transition-colors group-hover:border-primary"
      >
        {movie.poster_path ? (
          <Image
            src={`${POSTER_BASE}${movie.poster_path}`}
            alt={`${movie.title} poster`}
            fill
            sizes="80px"
            className="object-cover"
          />
        ) : (
          <span className="flex h-full items-center justify-center px-1 text-center text-[10px] text-muted-foreground">
            No poster
          </span>
        )}
      </Link>

      <div className="flex min-w-0 flex-1 flex-col">
        <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
          <span className={movie.media_type === "tv" ? "text-primary" : undefined}>{kind}</span>
          {movie.release_year > 0 && <span>· {movie.release_year}</span>}
          {movie.vote_average > 0 && (
            <span className="flex items-center gap-0.5">
              · <Star className="size-2.5 fill-gold text-gold" strokeWidth={0} /> {movie.vote_average.toFixed(1)}
            </span>
          )}
        </p>
        <h3 className="mt-1 font-heading text-base font-semibold leading-tight text-foreground">
          <Link href={`/movie/${movie.id}`} className="transition-colors hover:text-primary">
            {movie.title}
          </Link>
        </h3>
        <p className="mt-1.5 font-serif text-sm leading-snug text-muted-foreground">{pick.reason}</p>

        <div className="mt-auto flex items-end justify-between gap-3 pt-3">
          <div
            className="flex items-center gap-2"
            title={
              pick.similarity != null
                ? `Cosine similarity ${pick.similarity.toFixed(2)} from ${TOOL_NAMES[pick.source] ?? pick.source}`
                : `From ${TOOL_NAMES[pick.source] ?? pick.source}`
            }
          >
            <span className="flex items-end gap-0.5" aria-hidden="true">
              {[1, 2, 3, 4, 5].map((bar) => (
                <span
                  key={bar}
                  className={`w-1 ${bar <= confidence.level ? "bg-primary" : "bg-border"}`}
                  style={{ height: `${4 + bar * 2}px` }}
                />
              ))}
            </span>
            <span className="whitespace-nowrap font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
              {confidence.label}
            </span>
          </div>

          {token && (
            <div className="flex shrink-0 gap-1">
              <button
                type="button"
                onClick={() => give("like")}
                disabled={pending}
                aria-pressed={feedback === "like"}
                aria-label={`Like ${movie.title}`}
                className={`flex size-8 items-center justify-center border transition-colors disabled:opacity-50 ${
                  feedback === "like"
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border text-muted-foreground hover:border-primary hover:text-primary"
                }`}
              >
                <Heart className="size-3.5" fill={feedback === "like" ? "currentColor" : "none"} />
              </button>
              <button
                type="button"
                onClick={() => give("dislike")}
                disabled={pending}
                aria-pressed={feedback === "dislike"}
                aria-label={`Not for me: ${movie.title}`}
                className={`flex size-8 items-center justify-center border transition-colors disabled:opacity-50 ${
                  feedback === "dislike"
                    ? "border-foreground bg-foreground text-background"
                    : "border-border text-muted-foreground hover:border-foreground hover:text-foreground"
                }`}
              >
                <ThumbsDown className="size-3.5" />
              </button>
            </div>
          )}
        </div>
      </div>
    </article>
  );
}
