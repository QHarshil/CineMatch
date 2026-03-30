"use client";

import { useRef, useState, useEffect, useCallback } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import Link from "next/link";
import type { Movie } from "@/types/movie";
import { MovieCard } from "@/components/movie-card";

interface ScrollRowProps {
  title: string;
  movies: Movie[];
  seeAllHref?: string;
}

export function ScrollRow({ title, movies, seeAllHref }: ScrollRowProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const checkScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 0);
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
  }, []);

  useEffect(() => {
    checkScroll();
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener("scroll", checkScroll, { passive: true });
    window.addEventListener("resize", checkScroll);
    return () => {
      el.removeEventListener("scroll", checkScroll);
      window.removeEventListener("resize", checkScroll);
    };
  }, [checkScroll, movies]);

  function scroll(direction: "left" | "right") {
    const el = scrollRef.current;
    if (!el) return;
    const cardWidth = el.firstElementChild
      ? (el.firstElementChild as HTMLElement).offsetWidth + 16
      : 200;
    const distance = cardWidth * 3;
    el.scrollBy({
      left: direction === "left" ? -distance : distance,
      behavior: "smooth",
    });
  }

  if (movies.length === 0) return null;

  return (
    <section className="relative">
      {/* Section header */}
      <div className="flex items-center justify-between mb-4 px-4 lg:px-0">
        <h2 className="text-lg font-semibold">{title}</h2>
        {seeAllHref && (
          <Link
            href={seeAllHref}
            className="text-sm text-muted-foreground hover:text-gold transition-colors duration-200"
          >
            See all &rarr;
          </Link>
        )}
      </div>

      <div className="relative group/row">
        {/* Left fade + arrow */}
        {canScrollLeft && (
          <button
            onClick={() => scroll("left")}
            className="absolute left-0 top-0 bottom-0 z-10 w-12 flex items-center justify-center bg-gradient-to-r from-background to-transparent opacity-0 group-hover/row:opacity-100 transition-opacity duration-200"
            aria-label="Scroll left"
          >
            <ChevronLeft className="w-5 h-5 text-foreground" strokeWidth={1.5} />
          </button>
        )}

        {/* Scroll container */}
        <div
          ref={scrollRef}
          className="flex gap-4 overflow-x-auto scrollbar-hide px-4 lg:px-0 snap-x snap-mandatory"
        >
          {movies.map((movie) => (
            <div
              key={movie.id}
              className="w-[140px] sm:w-[160px] lg:w-[180px] shrink-0 snap-start"
            >
              <MovieCard movie={movie} />
            </div>
          ))}
        </div>

        {/* Right fade + arrow */}
        {canScrollRight && (
          <button
            onClick={() => scroll("right")}
            className="absolute right-0 top-0 bottom-0 z-10 w-12 flex items-center justify-center bg-gradient-to-l from-background to-transparent opacity-0 group-hover/row:opacity-100 transition-opacity duration-200"
            aria-label="Scroll right"
          >
            <ChevronRight className="w-5 h-5 text-foreground" strokeWidth={1.5} />
          </button>
        )}
      </div>
    </section>
  );
}
