import { notFound } from "next/navigation";
import Image from "next/image";
import { fetchMovieById } from "@/lib/api";
import { InteractionButtons } from "./interaction-buttons";

export const dynamic = "force-dynamic";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w500";
const TMDB_BACKDROP_BASE = "https://image.tmdb.org/t/p/w1280";

export default async function MovieDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let movie;
  try {
    movie = await fetchMovieById(id);
  } catch {
    notFound();
  }

  const posterUrl = movie.poster_path
    ? `${TMDB_IMAGE_BASE}${movie.poster_path}`
    : null;

  const backdropUrl = movie.backdrop_path
    ? `${TMDB_BACKDROP_BASE}${movie.backdrop_path}`
    : posterUrl;

  return (
    <div className="-mt-14">
      {/* Backdrop header */}
      <div className="relative w-full h-[50vh] min-h-[360px] overflow-hidden">
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
        <div className="absolute inset-0 bg-gradient-to-t from-background via-background/70 to-background/30" />
      </div>

      {/* Content */}
      <div className="mx-auto max-w-4xl px-4 lg:px-8 -mt-32 relative z-10 pb-16">
        <div className="flex flex-col sm:flex-row gap-8">
          {/* Poster */}
          <div className="relative w-48 sm:w-56 shrink-0 aspect-[2/3] overflow-hidden bg-surface">
            {posterUrl ? (
              <Image
                src={posterUrl}
                alt={`${movie.title} poster`}
                fill
                sizes="(max-width: 640px) 192px, 224px"
                className="object-cover"
                priority
              />
            ) : (
              <div className="flex h-full items-center justify-center text-muted-foreground text-sm">
                No poster
              </div>
            )}
          </div>

          {/* Details */}
          <div className="flex flex-col gap-4 pt-2">
            <div>
              <h1 className="font-heading text-4xl font-bold leading-tight">
                {movie.title}
              </h1>
              <div className="flex items-center gap-3 mt-2 text-sm text-muted-foreground">
                <span>{movie.release_year}</span>
                {movie.runtime > 0 && <span>{movie.runtime} min</span>}
                {movie.vote_average > 0 && (
                  <span className="text-gold font-medium">
                    {movie.vote_average.toFixed(1)}
                  </span>
                )}
              </div>
            </div>

            {movie.genres.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {movie.genres.map((genre) => (
                  <span
                    key={genre}
                    className="px-3 py-1 text-xs text-muted-foreground border border-border"
                  >
                    {genre}
                  </span>
                ))}
              </div>
            )}

            <div className="w-full h-px bg-border" />

            <p className="text-sm leading-relaxed text-foreground/70">
              {movie.overview || "No overview available."}
            </p>

            <div className="w-full h-px bg-border" />

            <InteractionButtons movieId={movie.id} />
          </div>
        </div>
      </div>
    </div>
  );
}
