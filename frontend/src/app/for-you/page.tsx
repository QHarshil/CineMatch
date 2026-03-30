"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { fetchRecommendations } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";
import { Skeleton } from "@/components/ui/skeleton";
import type { Movie } from "@/types/movie";
import Link from "next/link";
import { Button } from "@/components/ui/button";

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

  // Still resolving auth or fetching recommendations.
  const loading = authLoading || (session !== null && !fetched && !error);

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <h1 className="text-2xl font-bold mb-8">For You</h1>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
          {Array.from({ length: 10 }).map((_, i) => (
            <Skeleton key={i} className="aspect-[2/3] rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-4">
        <p className="text-muted-foreground">
          Sign in to get personalised recommendations.
        </p>
        <Link href="/login">
          <Button>Sign in</Button>
        </Link>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-24">
        <p className="text-destructive">Failed to load recommendations.</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold">For You</h1>
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
