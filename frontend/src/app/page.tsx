import { fetchMovies } from "@/lib/api";
import { Hero } from "@/components/hero";
import { ScrollRow } from "@/components/scroll-row";
import { SearchBar } from "@/components/search-bar";
import Link from "next/link";
import type { Movie } from "@/types/movie";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  let movies: Movie[] = [];
  try {
    movies = await fetchMovies(40, 0);
  } catch {
    // API unavailable — render landing without movie data
  }

  const featured = movies[0] ?? null;
  const trending = movies.slice(0, 20);
  const recent = movies.slice(20, 40);

  return (
    <div className="-mt-14">
      {/* Hero with featured movie backdrop, or static fallback */}
      {featured ? (
        <Hero movie={featured} />
      ) : (
        <section className="relative w-full h-[70vh] min-h-[480px] max-h-[720px] flex items-end bg-surface">
          <div className="px-4 pb-12 lg:px-12 max-w-2xl">
            <h1 className="font-heading text-4xl sm:text-5xl lg:text-6xl font-bold leading-tight mb-3">
              Find your next favourite film
            </h1>
            <p className="text-sm text-foreground/70 leading-relaxed mb-6">
              CineMatch uses a two-stage ML pipeline to surface movies you will
              actually enjoy, not just what is popular.
            </p>
          </div>
        </section>
      )}

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
