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
      <div className="mb-4 flex items-center justify-between border-b border-border pb-2.5">
        <h3 className="font-heading text-sm font-semibold uppercase tracking-[0.12em] text-foreground">
          {title}
        </h3>
        {seeAllHref && (
          <Link
            href={seeAllHref}
            className="eyebrow text-muted-foreground transition-colors duration-200 hover:text-primary"
          >
            See all
          </Link>
        )}
      </div>

      <div className="group/row relative">
        {/* Left fade + arrow */}
        {canScrollLeft && (
          <button
            onClick={() => scroll("left")}
            className="absolute bottom-0 left-0 top-0 z-10 flex w-12 items-center justify-center bg-gradient-to-r from-background to-transparent opacity-0 transition-opacity duration-200 group-hover/row:opacity-100"
            aria-label="Scroll left"
          >
            <span className="grid size-9 place-items-center border border-border bg-background">
              <ChevronLeft className="h-5 w-5 text-primary" strokeWidth={1.5} />
            </span>
          </button>
        )}

        {/* Scroll container */}
        <div
          ref={scrollRef}
          className="flex snap-x snap-mandatory gap-4 overflow-x-auto scrollbar-hide"
        >
          {movies.map((movie) => (
            <div
              key={movie.id}
              className="w-[140px] shrink-0 snap-start sm:w-[160px] lg:w-[180px]"
            >
              <MovieCard movie={movie} />
            </div>
          ))}
        </div>

        {/* Right fade + arrow */}
        {canScrollRight && (
          <button
            onClick={() => scroll("right")}
            className="absolute bottom-0 right-0 top-0 z-10 flex w-12 items-center justify-center bg-gradient-to-l from-background to-transparent opacity-0 transition-opacity duration-200 group-hover/row:opacity-100"
            aria-label="Scroll right"
          >
            <span className="grid size-9 place-items-center border border-border bg-background">
              <ChevronRight className="h-5 w-5 text-primary" strokeWidth={1.5} />
            </span>
          </button>
        )}
      </div>
    </section>
  );
}
