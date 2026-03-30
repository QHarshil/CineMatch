import Image from "next/image";
import Link from "next/link";
import type { Movie } from "@/types/movie";

const TMDB_BACKDROP_BASE = "https://image.tmdb.org/t/p/w1280";

interface HeroProps {
  movie: Movie;
}

export function Hero({ movie }: HeroProps) {
  const backdropUrl = movie.backdrop_path
    ? `${TMDB_BACKDROP_BASE}${movie.backdrop_path}`
    : movie.poster_path
      ? `${TMDB_BACKDROP_BASE}${movie.poster_path}`
      : null;

  return (
    <section className="relative w-full h-[70vh] min-h-[480px] max-h-[720px] overflow-hidden">
      {/* Backdrop image */}
      {backdropUrl && (
        <Image
          src={backdropUrl}
          alt=""
          fill
          sizes="100vw"
          className="object-cover"
          priority
        />
      )}

      {/* Gradient overlays */}
      <div className="absolute inset-0 bg-gradient-to-t from-background via-background/60 to-transparent" />
      <div className="absolute inset-0 bg-gradient-to-r from-background/80 to-transparent" />

      {/* Content - bottom left */}
      <div className="absolute bottom-0 left-0 right-0 px-4 pb-12 lg:px-12">
        <div className="max-w-2xl">
          {movie.genres.length > 0 && (
            <div className="flex items-center gap-2 mb-3">
              {movie.genres.slice(0, 3).map((genre) => (
                <span
                  key={genre}
                  className="text-xs text-muted-foreground uppercase tracking-widest"
                >
                  {genre}
                </span>
              ))}
            </div>
          )}

          <Link href={`/movie/${movie.id}`}>
            <h1 className="font-heading text-4xl sm:text-5xl lg:text-6xl font-bold leading-tight mb-3 hover:text-gold transition-colors duration-200">
              {movie.title}
            </h1>
          </Link>

          <div className="flex items-center gap-3 mb-4 text-sm text-muted-foreground">
            <span>{movie.release_year}</span>
            {movie.runtime > 0 && <span>{movie.runtime} min</span>}
            {movie.vote_average > 0 && (
              <span className="text-gold font-medium">
                {movie.vote_average.toFixed(1)}
              </span>
            )}
          </div>

          {movie.overview && (
            <p className="text-sm text-foreground/70 line-clamp-2 max-w-xl leading-relaxed mb-6">
              {movie.overview}
            </p>
          )}

          <Link
            href={`/movie/${movie.id}`}
            className="inline-block px-6 py-2.5 border border-gold text-gold text-sm font-medium tracking-wide hover:bg-gold hover:text-background transition-colors duration-200"
          >
            View details
          </Link>
        </div>
      </div>
    </section>
  );
}
