import { fetchMovies } from "@/lib/api";
import { SearchBar } from "@/components/search-bar";
import { BrowseGrid } from "./browse-grid";

export const dynamic = "force-dynamic";

export default async function BrowsePage() {
  const initialMovies = await fetchMovies(20, 0);

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <h1 className="text-2xl font-bold">Browse Movies</h1>
        <SearchBar />
      </div>
      <BrowseGrid initialMovies={initialMovies} />
    </div>
  );
}
