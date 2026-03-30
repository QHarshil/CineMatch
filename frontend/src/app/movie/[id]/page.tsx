import { notFound } from "next/navigation";
import Image from "next/image";
import { fetchMovieById } from "@/lib/api";

export const dynamic = "force-dynamic";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { InteractionButtons } from "./interaction-buttons";

const TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p/w500";

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

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <div className="flex flex-col sm:flex-row gap-8">
        {/* Poster */}
        <div className="relative w-full sm:w-64 shrink-0 aspect-[2/3] overflow-hidden rounded-lg bg-muted">
          {posterUrl ? (
            <Image
              src={posterUrl}
              alt={`${movie.title} poster`}
              fill
              sizes="(max-width: 640px) 100vw, 256px"
              className="object-cover"
              priority
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              No poster
            </div>
          )}
        </div>

        {/* Details */}
        <div className="flex flex-col gap-4">
          <div>
            <h1 className="text-3xl font-bold">{movie.title}</h1>
            <div className="flex items-center gap-3 mt-2 text-sm text-muted-foreground">
              <span>{movie.release_year}</span>
              {movie.runtime > 0 && <span>{movie.runtime} min</span>}
              {movie.vote_average > 0 && (
                <span className="flex items-center gap-1">
                  <span className="text-yellow-500">&#9733;</span>
                  {movie.vote_average.toFixed(1)}
                </span>
              )}
            </div>
          </div>

          {movie.genres.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {movie.genres.map((genre) => (
                <Badge key={genre} variant="secondary">
                  {genre}
                </Badge>
              ))}
            </div>
          )}

          <Separator />

          <p className="text-sm leading-relaxed text-muted-foreground">
            {movie.overview || "No overview available."}
          </p>

          <Separator />

          <InteractionButtons movieId={movie.id} />
        </div>
      </div>
    </div>
  );
}
