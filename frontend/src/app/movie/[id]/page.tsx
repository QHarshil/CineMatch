import { notFound } from "next/navigation";
import Image from "next/image";
import { fetchMovieById } from "@/lib/api";
import { createSupabaseServerClient } from "@/lib/supabase-server";
import { InteractionButtons } from "./interaction-buttons";
import { SimilarMoviesRow } from "./similar-movies-row";
import type { Movie } from "@/types/movie";

export const dynamic = "force-dynamic";

const TMDB_POSTER = "https://image.tmdb.org/t/p/w500";
const TMDB_BACKDROP = "https://image.tmdb.org/t/p/w1280";

const MOVIE_FIELDS =
  "id,tmdb_id,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime";

async function fetchSimilarMovies(movie: Movie): Promise<Movie[]> {
  try {
    const supabase = await createSupabaseServerClient();
    // Find movies sharing at least one genre, similar rating range, exclude self
    const { data } = await supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      .neq("id", movie.id)
      .overlaps("genres", movie.genres.slice(0, 2))
      .gte("vote_average", Math.max(0, movie.vote_average - 2))
      .lte("vote_average", Math.min(10, movie.vote_average + 2))
      .order("popularity", { ascending: false })
      .limit(15);
    return (data ?? []) as Movie[];
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

export default async function MovieDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let movie: Movie;
  try {
    movie = await fetchMovieById(id);
  } catch {
    notFound();
  }

  const posterUrl = movie.poster_path
    ? `${TMDB_POSTER}${movie.poster_path}`
    : null;

  const backdropUrl = movie.backdrop_path
    ? `${TMDB_BACKDROP}${movie.backdrop_path}`
    : null;

  const similarMovies = await fetchSimilarMovies(movie);

  return (
    <div className="-mt-14">
      {/* ── Backdrop hero ──────────────────────────────────────── */}
      <div className="relative w-full h-[55vh] min-h-[400px] overflow-hidden">
        {backdropUrl ? (
          <Image
            src={backdropUrl}
            alt=""
            fill
            sizes="100vw"
            className="object-cover"
            priority
          />
        ) : (
          <div className="absolute inset-0 bg-gradient-to-br from-gold/10 via-background to-background" />
        )}
        {/* Gradient overlays */}
        <div className="absolute inset-0 bg-gradient-to-t from-background via-background/60 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-r from-background/80 via-transparent to-transparent" />
      </div>

      {/* ── Content ────────────────────────────────────────────── */}
      <div className="mx-auto max-w-5xl px-4 lg:px-8 -mt-44 relative z-10 pb-8">
        <div className="flex flex-col sm:flex-row gap-8">
          {/* Poster */}
          <div className="relative w-40 sm:w-64 shrink-0 aspect-[2/3] overflow-hidden bg-surface rounded-lg shadow-xl shadow-black/40 mx-auto sm:mx-0">
            {posterUrl ? (
              <Image
                src={posterUrl}
                alt={`${movie.title} poster`}
                fill
                sizes="(max-width: 640px) 160px, 256px"
                className="object-cover rounded-lg"
                priority
              />
            ) : (
              <div className="flex h-full items-center justify-center text-muted-foreground text-sm rounded-lg">
                No poster
              </div>
            )}
          </div>

          {/* Metadata */}
          <div className="flex flex-col gap-4 pt-2 text-center sm:text-left">
            <div>
              <h1 className="font-heading text-3xl sm:text-4xl font-bold leading-tight">
                {movie.title}
              </h1>
              <div className="flex items-center justify-center sm:justify-start gap-3 mt-2 text-sm text-muted-foreground">
                <span>{movie.release_year}</span>
                {movie.runtime > 0 && (
                  <>
                    <span className="text-border">|</span>
                    <span>{formatRuntime(movie.runtime)}</span>
                  </>
                )}
              </div>
            </div>

            {/* Rating - prominent */}
            {movie.vote_average > 0 && (
              <div className="flex items-center justify-center sm:justify-start gap-1.5">
                <span className="text-gold text-lg">&#9733;</span>
                <span className="text-gold text-xl font-semibold">
                  {movie.vote_average.toFixed(1)}
                </span>
                <span className="text-muted-foreground text-sm ml-1">
                  / 10
                </span>
              </div>
            )}

            {/* Genre chips */}
            {movie.genres.length > 0 && (
              <div className="flex flex-wrap justify-center sm:justify-start gap-2">
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

            {/* Overview */}
            <p className="text-sm leading-relaxed text-foreground/70 max-w-xl line-clamp-4 sm:line-clamp-none">
              {movie.overview || "No overview available."}
            </p>
          </div>
        </div>
      </div>

      {/* ── Interaction buttons ────────────────────────────────── */}
      <div className="mx-auto max-w-5xl px-4 lg:px-8 py-6 border-t border-border/30">
        <InteractionButtons movieId={movie.id} />
      </div>

      {/* ── Similar movies ─────────────────────────────────────── */}
      {similarMovies.length > 0 && (
        <div className="mx-auto max-w-7xl px-4 lg:px-8 py-10">
          <SimilarMoviesRow movies={similarMovies} />
        </div>
      )}
    </div>
  );
}
