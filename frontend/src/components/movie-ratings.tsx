"use client";

import { useEffect, useState } from "react";
import { fetchMovieRatings } from "@/lib/api";

interface Scores {
  imdb: number | null;
  rt: number | null;
}

/**
 * Loads a title's IMDb and Rotten Tomatoes scores from OMDb (via the Go
 * backend) after render, so the detail page is never blocked on the lookup.
 * Renders nothing until a score arrives, so a title with no aggregate ratings
 * stays silent instead of flashing a loading state.
 */
export function MovieRatings({ movieId }: { movieId: string }) {
  const [scores, setScores] = useState<Scores | null>(null);

  useEffect(() => {
    let active = true;
    fetchMovieRatings(movieId)
      .then((r) => {
        if (active) setScores({ imdb: r.imdb_rating, rt: r.rt_rating });
      })
      .catch(() => {
        if (active) setScores({ imdb: null, rt: null });
      });
    return () => {
      active = false;
    };
  }, [movieId]);

  if (!scores || (scores.imdb === null && scores.rt === null)) return null;

  return (
    <div className="flex items-center justify-center gap-5 font-mono text-sm sm:justify-start">
      {scores.imdb !== null && (
        <span className="flex items-baseline gap-1.5">
          <span className="eyebrow text-primary">IMDb</span>
          <span className="text-foreground">{scores.imdb.toFixed(1)}</span>
        </span>
      )}
      {scores.rt !== null && (
        <span className="flex items-baseline gap-1.5">
          <span className="eyebrow text-primary">RT</span>
          <span className="text-foreground">{scores.rt}%</span>
        </span>
      )}
    </div>
  );
}
