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

  return (
    <Link href={`/movie/${movie.id}`} className="group flex flex-col gap-2">
      {/* Poster */}
      <div className="relative aspect-[2/3] w-full overflow-hidden bg-surface">
        {posterUrl ? (
          <Image
            src={posterUrl}
            alt={`${movie.title} poster`}
            fill
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 20vw"
            className="object-cover transition-opacity duration-200 ease-out group-hover:opacity-80"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
            No poster
          </div>
        )}
      </div>

      {/* Metadata below poster */}
      <div className="flex flex-col gap-0.5">
        <h3 className="font-heading text-base font-semibold leading-tight line-clamp-1 group-hover:text-gold transition-colors duration-200">
          {movie.title}
        </h3>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{movie.release_year}</span>
          {movie.genres.length > 0 && (
            <>
              <span className="text-border">|</span>
              <span className="line-clamp-1">
                {movie.genres.slice(0, 2).join(", ")}
              </span>
            </>
          )}
        </div>
        {movie.vote_average > 0 && (
          <span className="text-xs font-medium text-gold">
            {movie.vote_average.toFixed(1)}
          </span>
        )}
      </div>
    </Link>
  );
}
