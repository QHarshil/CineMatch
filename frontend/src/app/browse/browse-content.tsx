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
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      {/* Page heading */}
      <h1 className="font-heading text-2xl font-bold mb-6">
        {isSearchMode ? (
          <>
            Results for &lsquo;{searchQuery}&rsquo;
          </>
        ) : (
          "Browse"
        )}
      </h1>

      {/* Filter/sort bar — hidden in search mode */}
      {!isSearchMode && (
        <div className="flex flex-col sm:flex-row sm:items-center gap-4 mb-8">
          {/* Genre chips */}
          <div className="flex-1 overflow-x-auto scrollbar-hide">
            <div className="flex gap-2 pb-1">
              {["All", ...genres].map((genre) => (
                <button
                  key={genre}
                  onClick={() => handleGenreChange(genre)}
                  className={`shrink-0 px-3.5 py-1.5 text-xs transition-colors duration-200 ${
                    activeGenre === genre
                      ? "bg-gold text-background font-medium"
                      : "border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30"
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
              className="flex items-center gap-2 px-4 py-1.5 text-xs border border-border text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200"
            >
              {SORT_LABELS[sort]}
              <ChevronDown className="w-3.5 h-3.5" strokeWidth={1.5} />
            </button>
            {sortOpen && (
              <div className="absolute right-0 top-full mt-1 bg-surface border border-border z-50 min-w-[140px]">
                {(Object.entries(SORT_LABELS) as [SortOption, string][]).map(
                  ([key, label]) => (
                    <button
                      key={key}
                      onClick={() => handleSortChange(key)}
                      className={`block w-full text-left px-4 py-2 text-xs transition-colors duration-150 ${
                        sort === key
                          ? "text-gold bg-gold/5"
                          : "text-muted-foreground hover:text-foreground hover:bg-surface-hover"
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
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-5">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex flex-col gap-2">
              <div className="aspect-[2/3] bg-surface animate-pulse" />
              <div className="h-4 w-3/4 bg-surface animate-pulse" />
              <div className="h-3 w-1/3 bg-surface animate-pulse" />
            </div>
          ))}
        </div>
      )}

      {/* Movie grid */}
      {!loading && movies.length > 0 && (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-5">
            {movies.map((movie) => (
              <MovieCard key={movie.id} movie={movie} />
            ))}
          </div>

          {/* Load more */}
          {hasMore && !isSearchMode && (
            <div className="flex justify-center mt-12">
              <button
                onClick={loadMore}
                disabled={loadingMore}
                className="px-8 py-3 border border-border text-sm text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200 disabled:opacity-50"
              >
                {loadingMore ? "Loading..." : "Load more"}
              </button>
            </div>
          )}
        </>
      )}

      {/* Empty state */}
      {!loading && movies.length === 0 && (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <div className="w-16 h-16 border border-border flex items-center justify-center">
            <Film className="w-7 h-7 text-muted-foreground" strokeWidth={1.5} />
          </div>
          <h2 className="font-heading text-xl font-semibold">No movies found</h2>
          <p className="text-sm text-muted-foreground text-center max-w-xs">
            {isSearchMode
              ? "Try a different search term or browse by genre instead."
              : "Try a different genre or search term."}
          </p>
          <button
            onClick={clearFilters}
            className="mt-2 px-5 py-2 border border-gold text-gold text-sm hover:bg-gold hover:text-background transition-colors duration-200"
          >
            {isSearchMode ? "Browse all movies" : "Clear filters"}
          </button>
        </div>
      )}
    </div>
  );
}
