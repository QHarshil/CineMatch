import { searchMovies } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";

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
    <div className="mx-auto max-w-7xl px-4 pb-16 pt-24 lg:px-8">
      <p className="eyebrow text-primary">Search</p>
      <h1 className="mb-10 mt-2 font-heading text-3xl font-semibold uppercase tracking-tight">
        {query ? `Results for "${query}"` : "Search the catalog"}
      </h1>
      <MovieGrid movies={movies} />
    </div>
  );
}
