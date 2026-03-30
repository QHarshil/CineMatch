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
      className="group relative flex flex-col overflow-hidden rounded-lg bg-card transition-all duration-300 hover:-translate-y-1 hover:shadow-xl"
    >
      {/* Poster */}
      <div className="relative aspect-[2/3] w-full overflow-hidden bg-muted">
        {posterUrl ? (
          <Image
            src={posterUrl}
            alt={`${movie.title} poster`}
            fill
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 20vw"
            className="object-cover transition-transform duration-500 group-hover:scale-110"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
            No poster
          </div>
        )}

        {/* Gradient overlay - revealed on hover */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/40 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />

        {/* Hover content - slides up */}
        <div className="absolute bottom-0 left-0 right-0 p-3 translate-y-4 opacity-0 group-hover:translate-y-0 group-hover:opacity-100 transition-all duration-300">
          {movie.overview && (
            <p className="text-white/80 text-xs line-clamp-3 mb-2">
              {movie.overview}
            </p>
          )}
          {movie.genres.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {movie.genres.slice(0, 3).map((genre) => (
                <Badge
                  key={genre}
                  variant="secondary"
                  className="text-[10px] px-1.5 py-0 bg-white/20 text-white border-0 backdrop-blur-sm"
                >
                  {genre}
                </Badge>
              ))}
            </div>
          )}
        </div>

        {/* Rating badge - always visible on poster */}
        {movie.vote_average > 0 && (
          <div className="absolute top-2 right-2 flex items-center gap-0.5 bg-black/60 backdrop-blur-sm rounded-md px-1.5 py-0.5 text-xs font-medium text-white">
            <span className="text-yellow-400">&#9733;</span>
            {movie.vote_average.toFixed(1)}
          </div>
        )}
      </div>

      {/* Title bar below poster */}
      <div className="flex flex-col gap-0.5 p-2.5">
        <h3 className="font-medium leading-tight line-clamp-1 text-sm">
          {movie.title}
        </h3>
        <span className="text-xs text-muted-foreground">
          {movie.release_year}
        </span>
      </div>
    </Link>
  );
}
