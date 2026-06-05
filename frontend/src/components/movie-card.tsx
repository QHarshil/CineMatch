"use client";

import Image from "next/image";
import Link from "next/link";
import { Star } from "lucide-react";
import type { Movie } from "@/types/movie";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w342";

interface MovieCardProps {
  movie: Movie;
  matchScore?: number;
}

export function MovieCard({ movie, matchScore }: MovieCardProps) {
  const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}${movie.poster_path}`
    : null;

  const primaryGenre = movie.genres[0] ?? null;

  return (
    <Link href={`/movie/${movie.id}`} className="group flex flex-col">
      {/* Poster */}
      <div className="relative aspect-[2/3] w-full overflow-hidden border border-border bg-muted transition-colors duration-200 group-hover:border-primary">
        {posterUrl ? (
          <Image
            src={posterUrl}
            alt={`${movie.title} poster`}
            fill
            sizes="(max-width: 640px) 140px, 180px"
            className="object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            No poster
          </div>
        )}
        {matchScore != null && matchScore > 0.7 && (
          <span className="absolute right-0 top-0 bg-primary px-2 py-0.5 font-mono text-[10px] font-medium text-primary-foreground">
            {Math.round(matchScore * 100)}% match
          </span>
        )}
        {movie.media_type && (
          <span
            className={`absolute left-0 top-0 bg-background/85 px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider backdrop-blur ${
              movie.media_type === "tv" ? "text-primary" : "text-muted-foreground"
            }`}
          >
            {movie.media_type === "tv" ? "TV" : "Film"}
          </span>
        )}
      </div>

      {/* Always-visible metadata */}
      <div className="flex flex-col gap-1 pt-2.5">
        <h3 className="truncate font-heading text-sm font-medium leading-tight text-foreground transition-colors group-hover:text-primary">
          {movie.title}
        </h3>
        <div className="flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
          <span>{movie.release_year}</span>
          {movie.vote_average > 0 && (
            <>
              <span aria-hidden="true">·</span>
              <Star className="size-3 fill-gold text-gold" strokeWidth={0} />
              <span className="text-foreground/75">
                {movie.vote_average.toFixed(1)}
              </span>
            </>
          )}
        </div>
        {primaryGenre && (
          <span className="eyebrow mt-0.5 inline-block w-fit border border-border px-2 py-0.5 text-muted-foreground">
            {primaryGenre}
          </span>
        )}
      </div>
    </Link>
  );
}
