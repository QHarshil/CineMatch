import { createSupabaseServerClient } from "@/lib/supabase-server";
import { BrowseContent } from "./browse-content";

export const dynamic = "force-dynamic";

export const metadata = {
  title: "Browse | CineMatch",
  description: "Browse movies by genre, sort by popularity, rating, or release date.",
};

async function fetchGenres(): Promise<string[]> {
  try {
    const supabase = await createSupabaseServerClient();
    const { data } = await supabase
      .from("movies")
      .select("genres");
    if (!data) return [];

    const genreSet = new Set<string>();
    for (const row of data) {
      if (Array.isArray(row.genres)) {
        for (const g of row.genres) genreSet.add(g);
      }
    }
    return Array.from(genreSet).sort();
  } catch {
    return [];
  }
}

export default async function BrowsePage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string }>;
}) {
  const { q } = await searchParams;
  const genres = await fetchGenres();

  return <BrowseContent genres={genres} searchQuery={q ?? ""} />;
}
