import { createSupabaseServerClient } from "@/lib/supabase-server";
import { catalogGenres } from "@/lib/catalog";
import { BrowseContent } from "./browse-content";

export const dynamic = "force-dynamic";

export const metadata = {
  title: "Browse",
  description: "Browse movies and shows by genre, sorted by popularity, rating, or release date.",
};

async function fetchGenres(): Promise<string[]> {
  try {
    return await catalogGenres(await createSupabaseServerClient());
  } catch {
    return [];
  }
}

export default async function BrowsePage({ searchParams }: { searchParams: Promise<{ q?: string }> }) {
  const { q } = await searchParams;
  const genres = await fetchGenres();

  return <BrowseContent key={q ?? ""} genres={genres} searchQuery={q ?? ""} />;
}
