import { createSupabaseServerClient } from "@/lib/supabase-server";
import { Hero } from "@/components/hero";
import { ScrollRow } from "@/components/scroll-row";
import Link from "next/link";
import type { Movie } from "@/types/movie";

export const dynamic = "force-dynamic";

const MOVIE_FIELDS = "id,tmdb_id,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime";

async function fetchHomeData() {
  const supabase = await createSupabaseServerClient();

  const [trendingRes, topRatedRes, newReleasesRes] = await Promise.all([
    supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      .order("popularity", { ascending: false })
      .limit(20),
    supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      .order("vote_average", { ascending: false })
      .limit(20),
    supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      .order("release_year", { ascending: false })
      .limit(20),
  ]);

  return {
    trending: (trendingRes.data ?? []) as Movie[],
    topRated: (topRatedRes.data ?? []) as Movie[],
    newReleases: (newReleasesRes.data ?? []) as Movie[],
  };
}

export default async function HomePage() {
  let trending: Movie[] = [];
  let topRated: Movie[] = [];
  let newReleases: Movie[] = [];

  try {
    const data = await fetchHomeData();
    trending = data.trending;
    topRated = data.topRated;
    newReleases = data.newReleases;
  } catch {
    // Supabase unavailable — render empty landing
  }

  // Pick the highest-rated popular movie with a backdrop for the hero
  const featured =
    trending.find((m) => m.backdrop_path && m.vote_average >= 7) ??
    trending.find((m) => m.backdrop_path) ??
    trending[0] ??
    null;

  return (
    <div className="-mt-14">
      {/* Hero */}
      {featured ? (
        <Hero movie={featured} />
      ) : (
        <section className="relative w-full h-[70vh] min-h-[480px] max-h-[720px] flex items-end bg-surface">
          <div className="px-4 pb-12 lg:px-12 max-w-2xl">
            <h1 className="font-heading text-4xl sm:text-5xl lg:text-6xl font-bold leading-tight mb-3">
              Discover films you will love
            </h1>
            <p className="text-sm text-foreground/70 leading-relaxed">
              Personalised recommendations curated to your taste.
            </p>
          </div>
        </section>
      )}

      {/* Content rows */}
      <div className="mx-auto max-w-7xl space-y-12 py-12">
        {trending.length > 0 && (
          <ScrollRow
            title="Trending Now"
            movies={trending}
            seeAllHref="/browse"
          />
        )}

        {topRated.length > 0 && (
          <ScrollRow
            title="Top Rated"
            movies={topRated}
            seeAllHref="/browse"
          />
        )}

        {newReleases.length > 0 && (
          <ScrollRow
            title="New Releases"
            movies={newReleases}
            seeAllHref="/browse"
          />
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
