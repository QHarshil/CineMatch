"use client";

import { useRouter } from "next/navigation";
import { useState, useEffect, useRef, type FormEvent } from "react";
import { Search, Loader2 } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { Input } from "@/components/ui/input";
import { searchMovies } from "@/lib/api";
import type { Movie } from "@/types/movie";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w92";
const DEBOUNCE_MS = 300;

interface SearchBarProps {
  initialQuery?: string;
  variant?: "inline" | "hero" | "header";
}

export function SearchBar({ initialQuery = "", variant = "inline" }: SearchBarProps) {
  const router = useRouter();
  const [query, setQuery] = useState(initialQuery);
  const [results, setResults] = useState<Movie[]>([]);
  const [searching, setSearching] = useState(false);
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const trimmed = query.trim();
  const showDropdown = open && results.length > 0 && trimmed.length >= 2;

  useEffect(() => {
    if (trimmed.length < 2) return;

    const abortController = new AbortController();

    const timer = setTimeout(() => {
      setSearching(true);
      searchMovies(trimmed, 6)
        .then((movies) => {
          if (abortController.signal.aborted) return;
          setResults(movies);
          setOpen(movies.length > 0);
        })
        .catch(() => {
          if (!abortController.signal.aborted) setResults([]);
        })
        .finally(() => {
          if (!abortController.signal.aborted) setSearching(false);
        });
    }, DEBOUNCE_MS);

    return () => {
      clearTimeout(timer);
      abortController.abort();
    };
  }, [trimmed]);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
        setFocused(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (trimmed.length === 0) return;
    setOpen(false);
    setFocused(false);
    router.push(`/browse?q=${encodeURIComponent(trimmed)}`);
  }

  const isHero = variant === "hero";
  const isHeader = variant === "header";

  const widthClass = isHero
    ? "w-full max-w-lg"
    : isHeader
      ? `transition-all duration-300 ease-out ${focused ? "w-96" : "w-64"}`
      : "w-full max-w-sm";

  const inputHeight = isHero ? "h-12 text-base" : "h-9 text-sm";

  return (
    <div ref={containerRef} className={`relative ${widthClass}`}>
      <form onSubmit={handleSubmit}>
        <div className="relative">
          <div className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
            {searching ? (
              <Loader2 className="w-4 h-4 animate-spin" strokeWidth={1.5} />
            ) : (
              <Search className="w-4 h-4" strokeWidth={1.5} />
            )}
          </div>
          <Input
            type="search"
            placeholder="Search movies..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onFocus={() => {
              setFocused(true);
              if (results.length > 0 && trimmed.length >= 2) setOpen(true);
            }}
            onBlur={() => {
              // Delay blur so click on dropdown registers
              setTimeout(() => setFocused(false), 200);
            }}
            className={`pl-10 bg-surface border-border text-foreground placeholder:text-muted-foreground ${inputHeight}`}
          />
        </div>
      </form>

      {/* Live results dropdown */}
      {showDropdown && (
        <div className="absolute top-full left-0 right-0 mt-1 bg-surface border border-border z-50 overflow-hidden">
          {results.map((movie) => (
            <Link
              key={movie.id}
              href={`/movie/${movie.id}`}
              onClick={() => {
                setOpen(false);
                setFocused(false);
              }}
              className="flex items-center gap-3 px-3 py-2.5 hover:bg-surface-hover transition-colors duration-150"
            >
              {movie.poster_path ? (
                <Image
                  src={`${TMDB_IMAGE_BASE}${movie.poster_path}`}
                  alt=""
                  width={32}
                  height={48}
                  className="object-cover shrink-0"
                />
              ) : (
                <div className="w-8 h-12 bg-muted shrink-0" />
              )}
              <div className="flex flex-col min-w-0">
                <span className="font-heading text-sm font-semibold line-clamp-1">
                  {movie.title}
                </span>
                <span className="text-xs text-muted-foreground">
                  {movie.release_year}
                  {movie.vote_average > 0 && (
                    <span className="text-gold ml-2">
                      {movie.vote_average.toFixed(1)}
                    </span>
                  )}
                </span>
              </div>
            </Link>
          ))}
          <Link
            href={`/search?q=${encodeURIComponent(trimmed)}`}
            onClick={() => {
              setOpen(false);
              setFocused(false);
            }}
            className="block px-3 py-2.5 text-xs text-gold hover:bg-surface-hover transition-colors duration-150 border-t border-border"
          >
            View all results
          </Link>
        </div>
      )}
    </div>
  );
}
