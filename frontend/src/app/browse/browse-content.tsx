"use client";

import { useState, useCallback, useRef } from "react";
import { Film, ChevronDown } from "lucide-react";
import type { Movie } from "@/types/movie";
import { searchMovies } from "@/lib/api";
import { MovieCard } from "@/components/movie-card";
import { createSupabaseBrowserClient } from "@/lib/supabase-browser";

const PAGE_SIZE = 30;

type SortOption = "popular" | "top_rated" | "newest" | "a_z";

const SORT_LABELS: Record<SortOption, string> = {
  popular: "Popular",
  top_rated: "Top Rated",
  newest: "Newest",
  a_z: "A-Z",
};

const SORT_CONFIG: Record<SortOption, { column: string; ascending: boolean }> = {
  popular: { column: "popularity", ascending: false },
  top_rated: { column: "vote_average", ascending: false },
  newest: { column: "release_year", ascending: false },
  a_z: { column: "title", ascending: true },
};

interface BrowseContentProps {
  genres: string[];
  searchQuery: string;
}

export function BrowseContent({ genres, searchQuery }: BrowseContentProps) {
  const [activeGenre, setActiveGenre] = useState("All");
  const [sort, setSort] = useState<SortOption>("popular");
  const [movies, setMovies] = useState<Movie[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [sortOpen, setSortOpen] = useState(false);
  const sortRef = useRef<HTMLDivElement>(null);
  const initialFetchDone = useRef(false);
  const supabase = useRef(createSupabaseBrowserClient());

  const isSearchMode = searchQuery.length > 0;

  const fetchFromSupabase = useCallback(
    async (genre: string, sortKey: SortOption, offset: number) => {
      const { column, ascending } = SORT_CONFIG[sortKey];
      let query = supabase.current
        .from("movies")
        .select(
          "id,tmdb_id,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime"
        )
        .order(column, { ascending })
        .range(offset, offset + PAGE_SIZE - 1);

      if (genre !== "All") {
        query = query.contains("genres", [genre]);
      }

      const { data } = await query;
      return (data ?? []) as Movie[];
    },
    []
  );

  const loadInitial = useCallback(
    async (genre: string, sortKey: SortOption) => {
      setLoading(true);
      setHasMore(true);
      try {
        if (isSearchMode) {
          const results = await searchMovies(searchQuery, 40);
          setMovies(results);
          setHasMore(false);
        } else {
          const results = await fetchFromSupabase(genre, sortKey, 0);
          setMovies(results);
          setHasMore(results.length === PAGE_SIZE);
        }
      } catch {
        setMovies([]);
        setHasMore(false);
      } finally {
        setLoading(false);
      }
    },
    [isSearchMode, searchQuery, fetchFromSupabase]
  );

  // Trigger initial load
  if (!initialFetchDone.current) {
    initialFetchDone.current = true;
    loadInitial(activeGenre, sort);
  }

  function handleGenreChange(genre: string) {
    setActiveGenre(genre);
    initialFetchDone.current = true;
    loadInitial(genre, sort);
  }

  function handleSortChange(newSort: SortOption) {
    setSort(newSort);
    setSortOpen(false);
    initialFetchDone.current = true;
    loadInitial(activeGenre, newSort);
  }

  async function loadMore() {
    setLoadingMore(true);
    try {
      const results = await fetchFromSupabase(
        activeGenre,
        sort,
        movies.length
      );
      setMovies((prev) => [...prev, ...results]);
      setHasMore(results.length === PAGE_SIZE);
    } catch {
      setHasMore(false);
    } finally {
      setLoadingMore(false);
    }
  }

  function clearFilters() {
    setActiveGenre("All");
    setSort("popular");
    loadInitial("All", "popular");
  }

  return (
    <div className="mx-auto max-w-7xl px-4 pb-16 pt-24 lg:px-8">
      {/* Page heading */}
      <p className="eyebrow text-primary">{isSearchMode ? "Search" : "Catalog"}</p>
      <h1 className="mb-6 mt-2 font-heading text-3xl font-semibold uppercase tracking-tight">
        {isSearchMode ? <>Results for &lsquo;{searchQuery}&rsquo;</> : "Browse"}
      </h1>

      {/* Filter/sort bar — hidden in search mode */}
      {!isSearchMode && (
        <div className="mb-8 flex flex-col gap-4 border-y border-border py-3 sm:flex-row sm:items-center">
          {/* Genre chips */}
          <div className="flex-1 overflow-x-auto scrollbar-hide">
            <div className="flex gap-2 pb-1">
              {["All", ...genres].map((genre) => (
                <button
                  key={genre}
                  onClick={() => handleGenreChange(genre)}
                  className={`shrink-0 px-3.5 py-1.5 text-xs transition-colors duration-200 ${
                    activeGenre === genre
                      ? "bg-primary font-medium text-primary-foreground"
                      : "border border-border text-muted-foreground hover:border-primary hover:text-primary"
                  }`}
                >
                  {genre}
                </button>
              ))}
            </div>
          </div>

          {/* Sort dropdown */}
          <div ref={sortRef} className="relative shrink-0">
            <button
              onClick={() => setSortOpen(!sortOpen)}
              className="flex items-center gap-2 border border-border px-4 py-1.5 text-xs text-muted-foreground transition-colors duration-200 hover:border-primary hover:text-primary"
            >
              {SORT_LABELS[sort]}
              <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.5} />
            </button>
            {sortOpen && (
              <div className="absolute right-0 top-full z-50 mt-1 min-w-[140px] border border-border bg-popover shadow-sm">
                {(Object.entries(SORT_LABELS) as [SortOption, string][]).map(
                  ([key, label]) => (
                    <button
                      key={key}
                      onClick={() => handleSortChange(key)}
                      className={`block w-full px-4 py-2 text-left text-xs transition-colors duration-150 ${
                        sort === key
                          ? "bg-accent text-primary"
                          : "text-muted-foreground hover:bg-surface-hover hover:text-foreground"
                      }`}
                    >
                      {label}
                    </button>
                  )
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Loading skeleton */}
      {loading && (
        <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex flex-col gap-2">
              <div className="aspect-[2/3] animate-pulse bg-muted" />
              <div className="h-4 w-3/4 animate-pulse bg-muted" />
              <div className="h-3 w-1/3 animate-pulse bg-muted" />
            </div>
          ))}
        </div>
      )}

      {/* Movie grid */}
      {!loading && movies.length > 0 && (
        <>
          <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            {movies.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>

          {/* Load more */}
          {hasMore && !isSearchMode && (
            <div className="mt-12 flex justify-center">
              <button
                onClick={loadMore}
                disabled={loadingMore}
                className="eyebrow border border-border px-8 py-3 text-muted-foreground transition-colors duration-200 hover:border-primary hover:text-primary disabled:opacity-50"
              >
                {loadingMore ? "Loading..." : "Load more"}
              </button>
            </div>
          )}
        </>
      )}

      {/* Empty state */}
      {!loading && movies.length === 0 && (
        <div className="flex flex-col items-center justify-center gap-4 py-20">
          <div className="flex size-16 items-center justify-center border border-border text-primary">
            <Film className="size-7" strokeWidth={1.5} />
          </div>
          <h2 className="font-heading text-xl font-semibold uppercase tracking-tight">
            No titles found
          </h2>
          <p className="max-w-xs text-center font-serif text-muted-foreground">
            {isSearchMode
              ? "Try a different search term or browse by genre instead."
              : "Try a different genre or search term."}
          </p>
          <button
            onClick={clearFilters}
            className="eyebrow mt-2 border border-primary px-5 py-2.5 text-primary transition-colors duration-200 hover:bg-primary hover:text-primary-foreground"
          >
            {isSearchMode ? "Browse all titles" : "Clear filters"}
          </button>
        </div>
      )}
    </div>
  );
}
