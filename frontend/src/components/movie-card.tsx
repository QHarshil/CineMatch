"use client";

import Image from "next/image";
import Link from "next/link";
import type { Movie } from "@/types/movie";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w342";

interface MovieCardProps {
  movie: Movie;
}

export function MovieCard({ movie }: MovieCardProps) {
  const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}${movie.poster_path}`
    : null;

  const primaryGenre = movie.genres[0] ?? null;

  return (
    <Link
      href={`/movie/${movie.id}`}
      className="group flex flex-col transition-transform duration-200 ease-out hover:scale-[1.03] hover:shadow-lg hover:shadow-black/40"
    >
      {/* Poster */}
      <div className="relative aspect-[2/3] w-full overflow-hidden bg-surface">
        {posterUrl ? (
          <Image
            src={posterUrl}
            alt={`${movie.title} poster`}
            fill
            sizes="(max-width: 640px) 140px, 180px"
            className="object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-muted-foreground text-xs">
            No poster
          </div>
        )}
      </div>

      {/* Always-visible metadata */}
      <div className="flex flex-col gap-0.5 pt-2">
        <h3 className="font-heading text-sm font-semibold leading-tight truncate">
          {movie.title}
        </h3>
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span>{movie.release_year}</span>
          {movie.vote_average > 0 && (
            <>
              <span className="text-gold">&#9733;</span>
              <span className="text-gold font-medium">
                {movie.vote_average.toFixed(1)}
              </span>
            </>
          )}
        </div>
        {primaryGenre && (
          <span className="mt-0.5 inline-block w-fit text-[10px] px-2 py-0.5 text-muted-foreground border border-border">
            {primaryGenre}
          </span>
        )}
      </div>
    </Link>
  );
}
