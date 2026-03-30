import { fetchMovies } from "@/lib/api";
import { BrowseGrid } from "./browse-grid";

export const dynamic = "force-dynamic";

export default async function BrowsePage() {
  const initialMovies = await fetchMovies(20, 0);

  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <h1 className="font-heading text-3xl font-semibold mb-10">Browse</h1>
      <BrowseGrid initialMovies={initialMovies} />
    </div>
  );
}
