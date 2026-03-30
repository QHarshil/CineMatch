"use client";

import { useState, useTransition } from "react";
import type { Movie } from "@/types/movie";
import { fetchMovies } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";

const PAGE_SIZE = 20;

export function BrowseGrid({ initialMovies }: { initialMovies: Movie[] }) {
  const [movies, setMovies] = useState(initialMovies);
  const [hasMore, setHasMore] = useState(initialMovies.length === PAGE_SIZE);
  const [isPending, startTransition] = useTransition();

  function loadMore() {
    startTransition(async () => {
      const nextPage = await fetchMovies(PAGE_SIZE, movies.length);
      if (nextPage.length < PAGE_SIZE) {
        setHasMore(false);
      }
      setMovies((prev) => [...prev, ...nextPage]);
    });
  }

  return (
    <>
      <MovieGrid movies={movies} />
      {hasMore && (
        <div className="flex justify-center mt-12">
          <button
            onClick={loadMore}
            disabled={isPending}
            className="px-8 py-3 border border-border text-sm text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200 disabled:opacity-50"
          >
            {isPending ? "Loading..." : "Load more"}
          </button>
        </div>
      )}
    </>
  );
}
