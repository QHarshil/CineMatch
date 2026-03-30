"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { fetchRecommendations } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";
import type { Movie } from "@/types/movie";
import Link from "next/link";

export default function ForYouPage() {
  const { session, loading: authLoading } = useAuth();
  const [movies, setMovies] = useState<Movie[]>([]);
  const [source, setSource] = useState<string>("");
  const [fetched, setFetched] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading || !session) return;

    let cancelled = false;
    fetchRecommendations(session.access_token)
      .then((res) => {
        if (cancelled) return;
        setMovies(res.movies);
        setSource(res.source);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      })
      .finally(() => {
        if (!cancelled) setFetched(true);
      });

    return () => {
      cancelled = true;
    };
  }, [session, authLoading]);

  const loading = authLoading || (session !== null && !fetched && !error);

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
        <h1 className="font-heading text-3xl font-semibold mb-10">For You</h1>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-x-4 gap-y-8">
          {Array.from({ length: 10 }).map((_, i) => (
            <div key={i} className="flex flex-col gap-2">
              <div className="aspect-[2/3] bg-surface animate-pulse" />
              <div className="h-4 w-3/4 bg-surface animate-pulse" />
              <div className="h-3 w-1/3 bg-surface animate-pulse" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4 px-4">
        <p className="text-muted-foreground text-sm">
          Sign in to get personalised recommendations.
        </p>
        <Link
          href="/login"
          className="px-6 py-2.5 border border-gold text-gold text-sm hover:bg-gold hover:text-background transition-colors duration-200"
        >
          Sign in
        </Link>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center pt-32 pb-16">
        <p className="text-destructive text-sm">
          Failed to load recommendations.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <div className="flex items-center justify-between mb-10">
        <h1 className="font-heading text-3xl font-semibold">For You</h1>
        {source && (
          <span className="text-xs text-muted-foreground capitalize">
            {source.replace(/_/g, " ")}
          </span>
        )}
      </div>
      <MovieGrid movies={movies} />
    </div>
  );
}
