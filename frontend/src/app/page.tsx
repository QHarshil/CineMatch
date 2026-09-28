import Image from "next/image";
import Link from "next/link";
import { createSupabaseServerClient } from "@/lib/supabase-server";
import { LandingHero } from "@/components/landing/hero";
import { AssistantStory } from "@/components/landing/assistant-story";
import { MetricsBand, type Metric } from "@/components/landing/metrics-band";
import { ProductionGrid } from "@/components/landing/production-grid";
import { SplitHeading } from "@/components/motion/split-heading";
import { AGENT_EVAL, RANKER_EVAL, SEARCH_EVAL } from "@/lib/eval-results";
import { ScrollRow } from "@/components/scroll-row";
import { CodeTyper } from "@/components/landing/code-typer";
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

// Measured results; see src/lib/eval-results.ts for where each comes from.
const METRICS: Metric[] = [
  {
    value: RANKER_EVAL.lambdamartNdcg10,
    decimals: 3,
    label: "NDCG@10, LambdaMART",
    detail: `Re-ranker on held-out users, up ${RANKER_EVAL.liftOverPopularity}% on a popularity baseline.`,
  },
  {
    value: RANKER_EVAL.rerankP95Ms,
    decimals: 1,
    suffix: " ms",
    label: "p95 re-rank latency",
    detail: "Stage-two scoring of 50 candidates, measured locally.",
  },
  {
    value: SEARCH_EVAL.modes[2].descriptionP10,
    decimals: 2,
    label: "P@10, hybrid search",
    detail: `Paraphrased descriptions scored by genre, ${(SEARCH_EVAL.modes[2].descriptionP10 / SEARCH_EVAL.randomP10).toFixed(1)}x a random ranking.`,
  },
  {
    value: SEARCH_EVAL.modes[2].titleMrr,
    decimals: 2,
    label: "MRR@10, title lookups",
    detail: `Exact titles and typos. Keyword-only search scores ${SEARCH_EVAL.modes[0].descriptionP10.toFixed(2)} on descriptions.`,
  },
  {
    value: AGENT_EVAL.production.passed,
    suffix: `/${AGENT_EVAL.production.cases}`,
    label: "Agent eval cases passed",
    detail: `On ${AGENT_EVAL.production.model}, the production model: constraints, named titles, taste, vague and off-topic requests, and prompt injection.`,
  },
  {
    value: AGENT_EVAL.production.latencyP50S,
    decimals: 1,
    suffix: " s",
    label: "Median agent run",
    detail: `Tool calls, retrieval, and grounded picks end to end. Zero ungrounded picks shown; the server caught ${AGENT_EVAL.production.ungroundedCaught} attempt.`,
  },
];

async function fetchHomeData() {
  const supabase = await createSupabaseServerClient();

  const [trendingRes, topRatedRes, newReleasesRes, movieCountRes, seriesCountRes] = await Promise.all([
    supabase.from("movies").select(MOVIE_FIELDS).order("popularity", { ascending: false }).limit(20),
    supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      // Titles with a handful of votes top a raw rating sort, so rank the
      // well-rated ones by popularity instead.
      .gte("vote_average", 7.5)
      .order("popularity", { ascending: false })
      .limit(20),
    supabase.from("movies").select(MOVIE_FIELDS).order("release_year", { ascending: false }).limit(20),
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

  const featured =
    trending.find((m) => m.backdrop_path && m.vote_average >= 7) ??
    trending.find((m) => m.backdrop_path) ??
    trending[0] ??
    null;

  const backdropUrl = featured?.backdrop_path ? `${TMDB_BACKDROP_BASE}${featured.backdrop_path}` : null;

  const hasCatalog = trending.length > 0 || topRated.length > 0 || newReleases.length > 0;

  return (
    <div className="mx-auto max-w-6xl border-x border-border">
      <LandingHero catalogSize={movieCount + seriesCount} />

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
            <span className="eyebrow absolute bottom-4 right-5 z-10 text-white/90">CineMatch</span>
          </div>
        </div>
      </section>

      <section className="border-t border-border">
        <div className="px-6 pb-10 pt-14 lg:px-8">
          <p className="eyebrow text-primary">Inside the assistant</p>
          <SplitHeading
            text="One request, five steps, every one on the record."
            className="mt-4 max-w-3xl font-heading text-3xl font-semibold uppercase leading-[1.05] tracking-tight text-foreground sm:text-4xl"
          />
        </div>
        <div className="border-t border-border">
          <AssistantStory />
        </div>
      </section>

      <section className="border-t border-border">
        <div className="px-6 pb-10 pt-14 lg:px-8">
          <p className="eyebrow text-primary">Measured, not claimed</p>
          <SplitHeading
            text="Every number comes from an eval you can rerun."
            className="mt-4 max-w-3xl font-heading text-3xl font-semibold uppercase leading-[1.05] tracking-tight text-foreground sm:text-4xl"
          />
        </div>
        <MetricsBand metrics={METRICS} />
      </section>

      <section className="border-t border-border">
        <div className="px-6 pb-10 pt-14 lg:px-8">
          <p className="eyebrow text-primary">Built like production</p>
          <SplitHeading
            text="The parts you do not see in a demo."
            className="mt-4 max-w-3xl font-heading text-3xl font-semibold uppercase leading-[1.05] tracking-tight text-foreground sm:text-4xl"
          />
        </div>
        <ProductionGrid />
      </section>

      {hasCatalog && (
        <section className="border-t border-border px-6 py-14 lg:px-8">
          <p className="eyebrow text-primary">The catalog</p>
          <div className="mt-8 space-y-12">
            {trending.length > 0 && <ScrollRow title="Trending Now" movies={trending} seeAllHref="/browse" />}
            {topRated.length > 0 && <ScrollRow title="Top Rated" movies={topRated} seeAllHref="/browse" />}
            {newReleases.length > 0 && <ScrollRow title="New Releases" movies={newReleases} seeAllHref="/browse" />}
          </div>
        </section>
      )}

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

      <footer className="grid border-t border-border font-mono text-xs text-muted-foreground sm:grid-cols-3 sm:divide-x sm:divide-border">
        <div className="px-6 py-5">
          <span className="font-heading text-sm font-semibold uppercase tracking-tight text-foreground">CineMatch</span>
        </div>
        <div className="flex items-center px-6 py-5">NDCG@10 0.81 · ~0.9 ms re-rank</div>
        <div className="flex items-center px-6 py-5 sm:justify-end">Next.js · Go · pgvector · 2026</div>
      </footer>
    </div>
  );
}
