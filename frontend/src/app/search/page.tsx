import { searchMovies } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";
import { SearchBar } from "@/components/search-bar";

export const dynamic = "force-dynamic";

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string }>;
}) {
  const { q } = await searchParams;
  const query = q?.trim() ?? "";
  const movies = query.length > 0 ? await searchMovies(query, 40) : [];

  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-10">
        <h1 className="font-heading text-3xl font-semibold">
          {query ? `Results for "${query}"` : "Search"}
        </h1>
        <SearchBar initialQuery={query} />
      </div>
      <MovieGrid movies={movies} />
    </div>
  );
}
