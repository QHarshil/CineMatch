import { NextRequest, NextResponse } from "next/server";
import { createSupabaseServerClient } from "@/lib/supabase-server";

/**
 * GET /api/similar?movieId=<uuid>
 *
 * Fetches the seed movie's embedding from Supabase, then calls the
 * match_movies RPC to find the 5 nearest neighbors by cosine similarity.
 * Used by the "How it works" interactive demo.
 */
export async function GET(request: NextRequest) {
  const movieId = request.nextUrl.searchParams.get("movieId");
  if (!movieId) {
    return NextResponse.json({ error: "movieId required" }, { status: 400 });
  }

  const supabase = await createSupabaseServerClient();

  // Fetch the seed movie's embedding
  const { data: seedMovie, error: seedError } = await supabase
    .from("movies")
    .select("id, title, embedding")
    .eq("id", movieId)
    .single();

  if (seedError || !seedMovie?.embedding) {
    return NextResponse.json(
      { error: "Movie not found or has no embedding" },
      { status: 404 }
    );
  }

  // Call match_movies RPC with the seed embedding
  const { data: matches, error: matchError } = await supabase.rpc(
    "match_movies",
    {
      query_embedding: seedMovie.embedding,
      match_count: 6, // 5 neighbors + the seed itself
    }
  );

  if (matchError) {
    return NextResponse.json(
      { error: "Vector search failed" },
      { status: 500 }
    );
  }

  // Filter out the seed movie and take top 5
  const neighbors = (matches ?? [])
    .filter((m: { id: string }) => m.id !== movieId)
    .slice(0, 5)
    .map((m: { id: string; title: string; genres: string[]; vote_average: number; poster_path: string; similarity: number }) => ({
      id: m.id,
      title: m.title,
      genres: m.genres,
      vote_average: m.vote_average,
      poster_path: m.poster_path,
      similarity: m.similarity,
    }));

  return NextResponse.json({ seed: seedMovie.title, neighbors });
}
