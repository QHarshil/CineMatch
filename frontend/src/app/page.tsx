import Image from "next/image";
import Link from "next/link";
import { createSupabaseServerClient } from "@/lib/supabase-server";
import { LandingHero } from "@/components/landing-hero";
import { ScrollRow } from "@/components/scroll-row";
import { CodeTyper } from "@/components/code-typer";
import { discoverTitles } from "@/lib/api";
import type { Movie, SearchHit } from "@/types/movie";

export const dynamic = "force-dynamic";

const MOVIE_FIELDS =
  "id,tmdb_id,media_type,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime";

const TMDB_BACKDROP_BASE = "https://image.tmdb.org/t/p/w1280";

// The "see it in action" terminal runs this query against the live API.
const DEMO_QUERY = "mind-bending dream heist";

type TerminalLine = Parameters<typeof CodeTyper>[0]["lines"][number];

function demoSession(hits: SearchHit[], catalogSize: number): TerminalLine[] {
  const lines: TerminalLine[] = [
    [
      { t: "$ ", k: "punct" },
      { t: "cinematch ", k: "fn" },
      { t: "discover ", k: "keyword" },
      { t: `"${DEMO_QUERY}"`, k: "string" },
    ],
    [
      {
        t: catalogSize > 0 ? `hybrid retrieval over ${catalogSize.toLocaleString("en-US")} titles` : "hybrid retrieval",
        k: "comment",
      },
    ],
  ];
  hits.slice(0, 3).forEach((hit, i) => {
    const title = hit.title.length > 22 ? `${hit.title.slice(0, 21)}…` : hit.title;
    lines.push([
      { t: `${i + 1}  `, k: "punct" },
      { t: title.padEnd(24), k: "plain" },
      { t: hit.similarity != null ? `cos ${hit.similarity.toFixed(2)}` : "title match", k: "match" },
    ]);
  });
  return lines;
}

async function fetchDemoHits(): Promise<SearchHit[]> {
  try {
    const { results } = await discoverTitles(
      DEMO_QUERY,
      { limit: 3 },
      { next: { revalidate: 3600 }, signal: AbortSignal.timeout(2500) },
    );
    return results;
  } catch {
    return [];
  }
}

function catalogFeatures(movieCount: number, seriesCount: number) {
  const catalog =
    movieCount > 0
      ? `${movieCount.toLocaleString("en-US")} films and ${seriesCount.toLocaleString("en-US")} series, embedded and refreshed monthly from TMDB.`
      : "Films and series, embedded and refreshed monthly from TMDB.";
  return [
    { title: "Movies and TV", body: catalog },
    {
      title: "Search by meaning",
      body: "Describe a mood or a plot. Vector, keyword, and title matches are fused with reciprocal rank fusion.",
    },
    {
      title: "Ranks in milliseconds",
      body: "A LambdaMART model re-orders the 50 candidates with p95 latency near 0.9 ms.",
    },
    {
      title: "Honest metrics",
      body: "NDCG@10 0.81 on held-out users, a 14% lift over a popularity baseline.",
    },
  ];
}

async function fetchHomeData() {
  const supabase = await createSupabaseServerClient();

  const [trendingRes, topRatedRes, newReleasesRes, movieCountRes, seriesCountRes] = await Promise.all([
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
    supabase.from("movies").select("id", { count: "exact", head: true }).eq("media_type", "movie"),
    supabase.from("movies").select("id", { count: "exact", head: true }).eq("media_type", "tv"),
  ]);

  return {
    trending: (trendingRes.data ?? []) as Movie[],
    topRated: (topRatedRes.data ?? []) as Movie[],
    newReleases: (newReleasesRes.data ?? []) as Movie[],
    movieCount: movieCountRes.count ?? 0,
    seriesCount: seriesCountRes.count ?? 0,
  };
}

export default async function HomePage() {
  let trending: Movie[] = [];
  let topRated: Movie[] = [];
  let newReleases: Movie[] = [];
  let movieCount = 0;
  let seriesCount = 0;

  const demoHitsPromise = fetchDemoHits();
  try {
    const data = await fetchHomeData();
    trending = data.trending;
    topRated = data.topRated;
    newReleases = data.newReleases;
    movieCount = data.movieCount;
    seriesCount = data.seriesCount;
  } catch {
    // Supabase unavailable: render the pitch without catalog rows.
  }
  const demoHits = await demoHitsPromise;
  const features = catalogFeatures(movieCount, seriesCount);

  const featured =
    trending.find((m) => m.backdrop_path && m.vote_average >= 7) ??
    trending.find((m) => m.backdrop_path) ??
    trending[0] ??
    null;

  const backdropUrl = featured?.backdrop_path
    ? `${TMDB_BACKDROP_BASE}${featured.backdrop_path}`
    : null;

  const hasCatalog =
    trending.length > 0 || topRated.length > 0 || newReleases.length > 0;

  return (
    <div className="mx-auto max-w-6xl border-x border-border">
      <LandingHero />

      {/* See it in action */}
      <section className="halftone border-t border-border bg-wash">
        <div className="relative z-10 px-6 pt-12 lg:px-8">
          <p className="eyebrow text-primary">See it in action</p>
        </div>
        <div className="relative z-10 mt-6 grid border-t border-border lg:grid-cols-2">
          <div className="border-b border-border p-6 lg:border-b-0 lg:border-r lg:p-8">
            <CodeTyper
              filename="cinematch"
              lines={demoSession(demoHits, movieCount + seriesCount)}
              result={
                demoHits.length > 0
                  ? "live results, ranked by meaning, keywords, and title"
                  : "try any description in the search bar"
              }
              speed={26}
            />
          </div>
          <div className="duotone relative min-h-[300px]">
            {backdropUrl && (
              <Image
                src={backdropUrl}
                alt=""
                fill
                sizes="(max-width: 1024px) 100vw, 50vw"
                className="object-cover opacity-90 mix-blend-luminosity grayscale contrast-[1.05]"
              />
            )}
            <span className="eyebrow absolute bottom-4 right-5 z-10 text-white/90">
              CineMatch
            </span>
          </div>
        </div>
      </section>

      {/* Why it works */}
      <section className="border-t border-border">
        <div className="px-6 pt-12 lg:px-8">
          <p className="eyebrow text-primary">Why it works</p>
        </div>
        <div className="mt-6 grid gap-px border-t border-border bg-border sm:grid-cols-2">
          {features.map((feature) => (
            <div key={feature.title} className="bg-background p-6 lg:p-8">
              <h3 className="font-heading text-lg font-semibold uppercase tracking-wide text-foreground">
                {feature.title}
              </h3>
              <p className="mt-2 font-serif leading-relaxed text-muted-foreground">
                {feature.body}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Catalog */}
      {hasCatalog && (
        <section className="border-t border-border px-6 py-14 lg:px-8">
          <p className="eyebrow text-primary">The catalog</p>
          <div className="mt-8 space-y-12">
            {trending.length > 0 && (
              <ScrollRow title="Trending Now" movies={trending} seeAllHref="/browse" />
            )}
            {topRated.length > 0 && (
              <ScrollRow title="Top Rated" movies={topRated} seeAllHref="/browse" />
            )}
            {newReleases.length > 0 && (
              <ScrollRow
                title="New Releases"
                movies={newReleases}
                seeAllHref="/browse"
              />
            )}
          </div>
        </section>
      )}

      {/* Closing CTA */}
      <section className="halftone border-t border-border bg-wash">
        <div className="relative z-10 flex flex-col items-center px-6 py-20 text-center">
          <h2 className="max-w-2xl font-heading text-3xl font-semibold uppercase tracking-tight text-foreground sm:text-4xl">
            Find your next favorite.
          </h2>
          <p className="mt-3 max-w-md font-serif text-lg text-muted-foreground">
            One tap to sign in, no passwords. Your taste profile builds as you go.
          </p>
          <Link
            href="/login"
            className="eyebrow mt-8 bg-primary px-8 py-3 text-primary-foreground transition-colors hover:bg-primary/90"
          >
            Start matching
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="grid border-t border-border font-mono text-xs text-muted-foreground sm:grid-cols-3 sm:divide-x sm:divide-border">
        <div className="px-6 py-5">
          <span className="font-heading text-sm font-semibold uppercase tracking-tight text-foreground">
            CineMatch
          </span>
        </div>
        <div className="flex items-center px-6 py-5">
          NDCG@10 0.81 · ~0.9 ms re-rank
        </div>
        <div className="flex items-center px-6 py-5 sm:justify-end">
          Next.js · Go · pgvector · 2026
        </div>
      </footer>
    </div>
  );
}
