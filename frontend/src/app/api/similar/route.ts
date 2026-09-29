import { NextRequest, NextResponse } from "next/server";
import { createSupabaseServerClient } from "@/lib/supabase-server";
import { nearestTitles } from "@/lib/catalog";
import type { SimilarTitlesResponse } from "@/types/movie";

/**
 * GET /api/similar?movieId=<uuid>
 *
 * The five nearest titles to the seed by cosine similarity, for the How It
 * Works demo.
 */
export async function GET(request: NextRequest) {
  const movieId = request.nextUrl.searchParams.get("movieId");
  if (!movieId) {
    return NextResponse.json({ error: "movieId required" }, { status: 400 });
  }

  const { seedTitle, neighbors } = await nearestTitles(await createSupabaseServerClient(), movieId, 5);
  if (!seedTitle) {
    return NextResponse.json({ error: "Movie not found or has no embedding" }, { status: 404 });
  }

  const body: SimilarTitlesResponse = {
    seed: seedTitle,
    neighbors: neighbors.map(({ id, title, genres, vote_average, poster_path, similarity }) => ({
      id,
      title,
      genres,
      vote_average,
      poster_path,
      similarity,
    })),
  };
  return NextResponse.json(body);
}
