import { fetchMovies } from "@/lib/api";
import { SearchBar } from "@/components/search-bar";
import { BrowseGrid } from "./browse-grid";

export const dynamic = "force-dynamic";

export default async function BrowsePage() {
  const initialMovies = await fetchMovies(20, 0);

  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-10">
        <h1 className="font-heading text-3xl font-semibold">Browse</h1>
        <SearchBar />
      </div>
      <BrowseGrid initialMovies={initialMovies} />
    </div>
  );
}
