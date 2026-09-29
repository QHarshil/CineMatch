"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { Film, ChevronDown } from "lucide-react";
import type { Movie, RetrievalMode } from "@/types/movie";
import { discoverTitles } from "@/lib/api";
import { MovieCard } from "@/components/movie-card";
import { createSupabaseBrowserClient } from "@/lib/supabase-browser";
import { browseTitles, type CatalogSort } from "@/lib/catalog";

const PAGE_SIZE = 30;

type SortOption = CatalogSort;

const SORT_LABELS: Record<SortOption, string> = {
  popular: "Popular",
  top_rated: "Top Rated",
  newest: "Newest",
  a_z: "A-Z",
};

const RETRIEVAL_NOTES: Record<RetrievalMode, string> = {
  hybrid: "Matched by meaning, keywords, and title",
  keyword: "Matched by keywords and title",
  cached: "Showing title matches from the offline cache",
};

// Card width in the grid below: max-w-7xl, px-4 (px-8 from lg), gap-5, and
// 2, 3, 4, then 5 columns.
const GRID_CARD_SIZES =
  "(min-width: 1280px) 228px, (min-width: 1024px) calc(20vw - 29px), (min-width: 768px) calc(25vw - 23px), (min-width: 640px) calc(33.3vw - 24px), calc(50vw - 26px)";

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
  const [retrieval, setRetrieval] = useState<RetrievalMode | null>(null);
  const sortRef = useRef<HTMLDivElement>(null);
  const supabase = useRef(createSupabaseBrowserClient());

  const isSearchMode = searchQuery.length > 0;

  const fetchFromSupabase = useCallback(
    (genre: string, sortKey: SortOption, offset: number) =>
      browseTitles(supabase.current, {
        genre: genre === "All" ? undefined : genre,
        sort: sortKey,
        offset,
        limit: PAGE_SIZE,
      }),
    [],
  );

  const fetchFirstPage = useCallback(
    async (genre: string, sortKey: SortOption) => {
      try {
        if (isSearchMode) {
          const { results, retrieval: mode } = await discoverTitles(searchQuery, { limit: 40 });
          return { results, mode, more: false };
        }
        const results = await fetchFromSupabase(genre, sortKey, 0);
        return { results, mode: null, more: results.length === PAGE_SIZE };
      } catch {
        return { results: [] as Movie[], mode: null, more: false };
      }
    },
    [isSearchMode, searchQuery, fetchFromSupabase],
  );

  const showFirstPage = useCallback(({ results, mode, more }: Awaited<ReturnType<typeof fetchFirstPage>>) => {
    setMovies(results);
    setRetrieval(mode);
    setHasMore(more);
    setLoading(false);
  }, []);

  async function loadInitial(genre: string, sortKey: SortOption) {
    setLoading(true);
    showFirstPage(await fetchFirstPage(genre, sortKey));
  }

  // The page keys this component by query, so each search mounts it fresh in
  // the loading state; filter and sort changes call loadInitial directly.
  useEffect(() => {
    let cancelled = false;
    fetchFirstPage("All", "popular").then((page) => {
      if (!cancelled) showFirstPage(page);
    });
    return () => {
      cancelled = true;
    };
  }, [fetchFirstPage, showFirstPage]);

  function handleGenreChange(genre: string) {
    setActiveGenre(genre);
    loadInitial(genre, sort);
  }

  function handleSortChange(newSort: SortOption) {
    setSort(newSort);
    setSortOpen(false);
    loadInitial(activeGenre, newSort);
  }

  async function loadMore() {
    setLoadingMore(true);
    try {
      const results = await fetchFromSupabase(activeGenre, sort, movies.length);
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
      <p className="eyebrow text-primary">{isSearchMode ? "Search" : "Catalog"}</p>
      <h1 className="mb-6 mt-2 font-heading text-3xl font-semibold uppercase tracking-tight">
        {isSearchMode ? <>Results for &lsquo;{searchQuery}&rsquo;</> : "Browse"}
      </h1>
      {isSearchMode && retrieval && !loading && (
        <p className="eyebrow -mt-3 mb-8 text-muted-foreground">{RETRIEVAL_NOTES[retrieval]}</p>
      )}

      {!isSearchMode && (
        <div className="mb-8 flex flex-col gap-4 border-y border-border py-3 sm:flex-row sm:items-center">
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
                {(Object.entries(SORT_LABELS) as [SortOption, string][]).map(([key, label]) => (
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
                ))}
              </div>
            )}
          </div>
        </div>
      )}

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

      {!loading && movies.length > 0 && (
        <>
          <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            {movies.map((movie) => (
              <MovieCard key={movie.id} movie={movie} sizes={GRID_CARD_SIZES} />
            ))}
          </div>

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

      {!loading && movies.length === 0 && (
        <div className="flex flex-col items-center justify-center gap-4 py-20">
          <div className="flex size-16 items-center justify-center border border-border text-primary">
            <Film className="size-7" strokeWidth={1.5} />
          </div>
          <h2 className="font-heading text-xl font-semibold uppercase tracking-tight">No titles found</h2>
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
