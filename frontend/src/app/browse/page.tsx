import { fetchMovies } from "@/lib/api";
import { MovieGrid } from "@/components/movie-grid";
import { SearchBar } from "@/components/search-bar";

// Always fetch fresh data at request time — the movie catalog changes when we re-seed.
export const dynamic = "force-dynamic";

export default async function BrowsePage() {
  const movies = await fetchMovies(40, 0);

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <h1 className="text-2xl font-bold">Browse Movies</h1>
        <SearchBar />
      </div>
      <MovieGrid movies={movies} />
    </div>
  );
}
