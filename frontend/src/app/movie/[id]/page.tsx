import { notFound } from "next/navigation";
import Image from "next/image";
import { fetchMovieById } from "@/lib/api";
import { createSupabaseServerClient } from "@/lib/supabase-server";
import { InteractionButtons } from "./interaction-buttons";
import { ScrollRow } from "@/components/scroll-row";
import { MovieRatings } from "@/components/movie-ratings";
import { tmdbImage } from "@/lib/tmdb-image";
import type { Movie } from "@/types/movie";

export const dynamic = "force-dynamic";

/** Nearest titles by embedding, the same kNN search the recommender uses. */
async function fetchSimilarMovies(movie: Movie): Promise<Movie[]> {
  try {
    const supabase = await createSupabaseServerClient();
    const { data: seed } = await supabase.from("movies").select("embedding").eq("id", movie.id).single();
    if (!seed?.embedding) return [];
    const { data: neighbors } = await supabase.rpc("match_movies", {
      query_embedding: seed.embedding,
      match_count: 16,
    });
    return ((neighbors ?? []) as Movie[]).filter((m) => m.id !== movie.id).slice(0, 15);
  } catch {
    return [];
  }
}

function formatRuntime(minutes: number): string {
  if (minutes <= 0) return "";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export default async function MovieDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let movie: Movie;
  try {
    movie = await fetchMovieById(id);
  } catch {
    notFound();
  }

  const posterUrl = movie.poster_path ? tmdbImage(movie.poster_path) : null;

  const backdropUrl = movie.backdrop_path ? tmdbImage(movie.backdrop_path) : null;

  const similarMovies = await fetchSimilarMovies(movie);

  return (
    <div className="-mt-16">
      <div className="relative h-[55vh] min-h-[400px] w-full overflow-hidden">
        {backdropUrl ? (
          <Image src={backdropUrl} alt="" fill sizes="100vw" className="object-cover" priority />
        ) : (
          <div className="absolute inset-0 bg-gradient-to-br from-primary/10 via-background to-background" />
        )}
        {/* Fade the still into the white page */}
        <div className="absolute inset-0 bg-gradient-to-t from-background via-background/55 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-r from-background/70 via-transparent to-transparent" />
      </div>

      <div className="relative z-10 mx-auto -mt-44 max-w-5xl px-4 pb-8 lg:px-8">
        <div className="flex flex-col gap-8 sm:flex-row">
          <div className="relative mx-auto aspect-[2/3] w-40 shrink-0 overflow-hidden border border-border bg-muted sm:mx-0 sm:w-64">
            {posterUrl ? (
              <Image
                src={posterUrl}
                alt={`${movie.title} poster`}
                fill
                sizes="(min-width: 640px) 256px, 160px"
                className="object-cover"
                priority
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">No poster</div>
            )}
          </div>

          <div className="flex flex-col gap-4 pt-2 text-center sm:text-left">
            <div>
              <h1 className="font-heading text-3xl font-semibold leading-tight sm:text-4xl">{movie.title}</h1>
              <div className="mt-2 flex items-center justify-center gap-3 font-mono text-sm text-muted-foreground sm:justify-start">
                <span>{movie.release_year}</span>
                {movie.runtime > 0 && (
                  <>
                    <span className="text-border">|</span>
                    <span>{formatRuntime(movie.runtime)}</span>
                  </>
                )}
                {movie.media_type && (
                  <>
                    <span className="text-border">|</span>
                    <span className="uppercase tracking-wider text-primary">
                      {movie.media_type === "tv" ? "Series" : "Film"}
                    </span>
                  </>
                )}
              </div>
            </div>

            {movie.vote_average > 0 && (
              <div className="flex items-center justify-center gap-1.5 sm:justify-start">
                <span className="text-lg text-gold">&#9733;</span>
                <span className="font-mono text-xl font-semibold text-foreground">{movie.vote_average.toFixed(1)}</span>
                <span className="ml-1 text-sm text-muted-foreground">/ 10</span>
              </div>
            )}

            {/* IMDb / Rotten Tomatoes from OMDb, loaded client-side */}
            <MovieRatings movieId={movie.id} />

            {movie.genres.length > 0 && (
              <div className="flex flex-wrap justify-center gap-2 sm:justify-start">
                {movie.genres.map((genre) => (
                  <span key={genre} className="eyebrow border border-border px-3 py-1 text-muted-foreground">
                    {genre}
                  </span>
                ))}
              </div>
            )}

            <p className="max-w-xl font-serif leading-relaxed text-muted-foreground line-clamp-4 sm:line-clamp-none">
              {movie.overview || "No overview available."}
            </p>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-5xl border-t border-border px-4 py-6 lg:px-8">
        <InteractionButtons movieId={movie.id} />
      </div>

      {similarMovies.length > 0 && (
        <div className="mx-auto max-w-7xl px-4 py-10 lg:px-8">
          <ScrollRow title="Similar Titles" movies={similarMovies} />
        </div>
      )}
    </div>
  );
}
