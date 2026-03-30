"use client";

import Image from "next/image";
import Link from "next/link";
import type { Movie } from "@/types/movie";
import { Badge } from "@/components/ui/badge";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w342";

interface MovieCardProps {
  movie: Movie;
}

export function MovieCard({ movie }: MovieCardProps) {
  const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}${movie.poster_path}`
    : null;

  return (
    <Link
      href={`/movie/${movie.id}`}
      className="group flex flex-col overflow-hidden rounded-lg border border-border bg-card transition-shadow hover:shadow-lg"
    >
      <div className="relative aspect-[2/3] w-full overflow-hidden bg-muted">
        {posterUrl ? (
          <Image
            src={posterUrl}
            alt={`${movie.title} poster`}
            fill
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 20vw"
            className="object-cover transition-transform group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
            No poster
          </div>
        )}
      </div>
      <div className="flex flex-col gap-1.5 p-3">
        <h3 className="font-medium leading-tight line-clamp-2 text-sm">
          {movie.title}
        </h3>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{movie.release_year}</span>
          {movie.vote_average > 0 && (
            <span className="flex items-center gap-0.5">
              <span className="text-yellow-500">&#9733;</span>
              {movie.vote_average.toFixed(1)}
            </span>
          )}
        </div>
        {movie.genres.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1">
            {movie.genres.slice(0, 2).map((genre) => (
              <Badge key={genre} variant="secondary" className="text-[10px] px-1.5 py-0">
                {genre}
              </Badge>
            ))}
          </div>
        )}
      </div>
    </Link>
  );
}
