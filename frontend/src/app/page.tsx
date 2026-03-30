import { fetchMovies } from "@/lib/api";
import { Hero } from "@/components/hero";
import { ScrollRow } from "@/components/scroll-row";
import { SearchBar } from "@/components/search-bar";
import Link from "next/link";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const movies = await fetchMovies(40, 0);

  const featured = movies[0];
  const trending = movies.slice(0, 20);
  const recent = movies.slice(20, 40);

  return (
    <div className="-mt-14">
      {/* Hero with featured movie backdrop */}
      {featured && <Hero movie={featured} />}

      {/* Content rows */}
      <div className="mx-auto max-w-7xl space-y-12 py-12">
        {/* Search */}
        <div className="flex flex-col items-center gap-3 px-4">
          <p className="text-sm text-muted-foreground">
            Find something specific
          </p>
          <SearchBar variant="hero" />
        </div>

        {/* Trending row */}
        {trending.length > 0 && (
          <ScrollRow title="Trending Now" movies={trending} />
        )}

        {/* More to explore */}
        {recent.length > 0 && (
          <ScrollRow title="More to Explore" movies={recent} />
        )}

        {/* Browse CTA */}
        <div className="flex justify-center px-4">
          <Link
            href="/browse"
            className="px-8 py-3 border border-border text-sm text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200"
          >
            Browse all movies
          </Link>
        </div>
      </div>
    </div>
  );
}
