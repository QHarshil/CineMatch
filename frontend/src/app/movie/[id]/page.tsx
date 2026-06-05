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
  "id,tmdb_id,media_type,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime";

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
    <div className="-mt-16">
      {/* ── Backdrop hero ──────────────────────────────────────── */}
      <div className="relative h-[55vh] min-h-[400px] w-full overflow-hidden">
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
          <div className="absolute inset-0 bg-gradient-to-br from-primary/10 via-background to-background" />
        )}
        {/* Fade the still into the white page */}
        <div className="absolute inset-0 bg-gradient-to-t from-background via-background/55 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-r from-background/70 via-transparent to-transparent" />
      </div>

      {/* ── Content ────────────────────────────────────────────── */}
      <div className="relative z-10 mx-auto -mt-44 max-w-5xl px-4 pb-8 lg:px-8">
        <div className="flex flex-col gap-8 sm:flex-row">
          {/* Poster */}
          <div className="relative mx-auto aspect-[2/3] w-40 shrink-0 overflow-hidden border border-border bg-muted sm:mx-0 sm:w-64">
            {posterUrl ? (
              <Image
                src={posterUrl}
                alt={`${movie.title} poster`}
                fill
                sizes="(max-width: 640px) 160px, 256px"
                className="object-cover"
                priority
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                No poster
              </div>
            )}
          </div>

          {/* Metadata */}
          <div className="flex flex-col gap-4 pt-2 text-center sm:text-left">
            <div>
              <h1 className="font-heading text-3xl font-semibold leading-tight sm:text-4xl">
                {movie.title}
              </h1>
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

            {/* Rating - prominent */}
            {movie.vote_average > 0 && (
              <div className="flex items-center justify-center gap-1.5 sm:justify-start">
                <span className="text-lg text-gold">&#9733;</span>
                <span className="font-mono text-xl font-semibold text-foreground">
                  {movie.vote_average.toFixed(1)}
                </span>
                <span className="ml-1 text-sm text-muted-foreground">/ 10</span>
              </div>
            )}

            {/* Genre chips */}
            {movie.genres.length > 0 && (
              <div className="flex flex-wrap justify-center gap-2 sm:justify-start">
                {movie.genres.map((genre) => (
                  <span
                    key={genre}
                    className="eyebrow border border-border px-3 py-1 text-muted-foreground"
                  >
                    {genre}
                  </span>
                ))}
              </div>
            )}

            {/* Overview */}
            <p className="max-w-xl font-serif leading-relaxed text-muted-foreground line-clamp-4 sm:line-clamp-none">
              {movie.overview || "No overview available."}
            </p>
          </div>
        </div>
      </div>

      {/* ── Interaction buttons ────────────────────────────────── */}
      <div className="mx-auto max-w-5xl border-t border-border px-4 py-6 lg:px-8">
        <InteractionButtons movieId={movie.id} />
      </div>

      {/* ── Similar movies ─────────────────────────────────────── */}
      {similarMovies.length > 0 && (
        <div className="mx-auto max-w-7xl px-4 py-10 lg:px-8">
          <SimilarMoviesRow movies={similarMovies} />
        </div>
      )}
    </div>
  );
}
