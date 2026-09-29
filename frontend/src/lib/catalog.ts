import type { SupabaseClient } from "@supabase/supabase-js";
import type { Movie, SimilarTitle } from "@/types/movie";

/**
 * Catalog reads that go straight to Supabase under RLS. Every page and route
 * calls these instead of building queries, so a schema change lands here.
 * Each function takes the client, so server and browser code share them.
 * Reads that fail return empty lists, and pages render their empty states.
 */

/** The columns of Movie. The embedding never leaves the database. */
export const MOVIE_FIELDS =
  "id,tmdb_id,media_type,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime,original_language";

export type CatalogSort = "popular" | "top_rated" | "newest" | "a_z";

const SORT_COLUMNS: Record<CatalogSort, { column: string; ascending: boolean }> = {
  popular: { column: "popularity", ascending: false },
  top_rated: { column: "vote_average", ascending: false },
  newest: { column: "release_year", ascending: false },
  a_z: { column: "title", ascending: true },
};

export interface BecauseYouLikedSection {
  likedMovie: Movie;
  similarMovies: Movie[];
}

async function rows<T>(query: PromiseLike<{ data: unknown }>): Promise<T[]> {
  const { data } = await query;
  return (data ?? []) as T[];
}

export function popularTitles(db: SupabaseClient, limit = 20): Promise<Movie[]> {
  return rows(db.from("movies").select(MOVIE_FIELDS).order("popularity", { ascending: false }).limit(limit));
}

/** Titles rated 7.5 or higher, most popular first. A raw rating sort surfaces titles with a handful of votes. */
export function wellRatedTitles(db: SupabaseClient, limit = 20): Promise<Movie[]> {
  return rows(
    db
      .from("movies")
      .select(MOVIE_FIELDS)
      .gte("vote_average", 7.5)
      .order("popularity", { ascending: false })
      .limit(limit),
  );
}

export function newestTitles(db: SupabaseClient, limit = 20): Promise<Movie[]> {
  return rows(db.from("movies").select(MOVIE_FIELDS).order("release_year", { ascending: false }).limit(limit));
}

export async function catalogCounts(db: SupabaseClient): Promise<{ movies: number; series: number }> {
  const count = (type: "movie" | "tv") =>
    db.from("movies").select("id", { count: "exact", head: true }).eq("media_type", type);
  const [movies, series] = await Promise.all([count("movie"), count("tv")]);
  return { movies: movies.count ?? 0, series: series.count ?? 0 };
}

export function browseTitles(
  db: SupabaseClient,
  { genre, sort, offset, limit }: { genre?: string; sort: CatalogSort; offset: number; limit: number },
): Promise<Movie[]> {
  const { column, ascending } = SORT_COLUMNS[sort];
  let query = db
    .from("movies")
    .select(MOVIE_FIELDS)
    .order(column, { ascending })
    .range(offset, offset + limit - 1);
  if (genre) query = query.contains("genres", [genre]);
  return rows(query);
}

export async function catalogGenres(db: SupabaseClient): Promise<string[]> {
  const genreRows = await rows<{ genres: string[] | null }>(db.from("movies").select("genres"));
  return [...new Set(genreRows.flatMap((r) => r.genres ?? []))].sort();
}

/** Titles sharing any of the genres, best rated first. Drives the signed-out demo profiles. */
export function titlesInGenres(db: SupabaseClient, genres: string[], limit = 20): Promise<Movie[]> {
  return rows(
    db
      .from("movies")
      .select(MOVIE_FIELDS)
      .overlaps("genres", genres)
      .order("vote_average", { ascending: false })
      .limit(limit),
  );
}

/** Popular titles with posters, for decorative poster walls. */
export function posterTitles(
  db: SupabaseClient,
  limit: number,
): Promise<Pick<Movie, "id" | "title" | "poster_path">[]> {
  return rows(
    db
      .from("movies")
      .select("id,title,poster_path")
      .not("poster_path", "is", null)
      .order("popularity", { ascending: false })
      .limit(limit),
  );
}

/**
 * Nearest titles to one title by embedding, through the same match_movies
 * kNN search the recommender uses. The seed title itself is left out.
 */
export async function nearestTitles(
  db: SupabaseClient,
  movieId: string,
  count: number,
): Promise<{ seedTitle: string | null; neighbors: SimilarTitle[] }> {
  const { data: seed } = await db.from("movies").select("title,embedding").eq("id", movieId).single();
  if (!seed?.embedding) return { seedTitle: null, neighbors: [] };
  const matches = await rows<SimilarTitle>(
    db.rpc("match_movies", { query_embedding: seed.embedding, match_count: count + 1 }),
  );
  return { seedTitle: seed.title as string, neighbors: matches.filter((m) => m.id !== movieId).slice(0, count) };
}

/**
 * One row per recently liked title: other titles sharing its top two genres
 * and rated within two points of it, with each title shown at most once.
 */
export async function becauseYouLiked(
  db: SupabaseClient,
  userId: string,
  likes = 3,
): Promise<BecauseYouLikedSection[]> {
  const liked = await rows<{ movie_id: string }>(
    db
      .from("interactions")
      .select("movie_id")
      .eq("user_id", userId)
      .eq("type", "like")
      .order("created_at", { ascending: false })
      .limit(likes),
  );
  if (liked.length === 0) return [];

  const likedIds = liked.map((l) => l.movie_id);
  const likedMovies = await rows<Movie>(db.from("movies").select(MOVIE_FIELDS).in("id", likedIds));
  const seen = new Set(likedIds);
  const sections: BecauseYouLikedSection[] = [];

  for (const movie of likedMovies) {
    const topGenres = movie.genres.slice(0, 2);
    if (topGenres.length === 0) continue;
    const similar = await rows<Movie>(
      db
        .from("movies")
        .select(MOVIE_FIELDS)
        .neq("id", movie.id)
        .overlaps("genres", topGenres)
        .gte("vote_average", Math.max(0, movie.vote_average - 2))
        .order("popularity", { ascending: false })
        .limit(20),
    );
    const fresh = similar.filter((m) => !seen.has(m.id));
    fresh.forEach((m) => seen.add(m.id));
    if (fresh.length > 0) sections.push({ likedMovie: movie, similarMovies: fresh });
  }
  return sections;
}
