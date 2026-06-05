import { createSupabaseServerClient } from "@/lib/supabase-server";
import { PipelineDiagram } from "./pipeline-diagram";
import { SimilarMoviesDemo } from "./similar-movies-demo";
import { SectionReveal } from "./section-reveal";
import {
  Database,
  Cpu,
  BarChart3,
  Layers,
  Zap,
  Globe,
  ExternalLink,
} from "lucide-react";
import Link from "next/link";

export const metadata = {
  title: "How It Works",
  description:
    "A technical deep-dive into how CineMatch builds personalized movie recommendations using vector search and learned ranking.",
};

const MOVIE_FIELDS = "id, title, poster_path" as const;

async function fetchSeedMovies() {
  try {
    const supabase = await createSupabaseServerClient();
    const { data } = await supabase
      .from("movies")
      .select(MOVIE_FIELDS)
      .order("popularity", { ascending: false })
      .limit(30);
    return (data ?? []).filter(
      (m: { poster_path: string | null }) => m.poster_path
    );
  } catch {
    return [];
  }
}

const FEATURE_WEIGHTS = [
  { name: "Cosine Similarity", weight: 0.50, description: "How close the movie is to the user's taste in embedding space" },
  { name: "Vote Quality", weight: 0.25, description: "TMDB community rating, normalized to a 0-1 scale" },
  { name: "Log Popularity", weight: 0.15, description: "Logarithmic popularity prevents blockbusters from drowning everything" },
  { name: "Genre Overlap", weight: 0.10, description: "Fraction of the movie's genres matching the user's preferences" },
];

const EVAL_RESULTS = [
  { model: "Popularity Baseline", ndcg: 0.72, mrr: 0.88, hitRate: 1.0 },
  { model: "Vector Retrieval Only", ndcg: 0.80, mrr: 0.94, hitRate: 1.0 },
  { model: "Linear Re-ranker", ndcg: 0.80, mrr: 0.95, hitRate: 1.0 },
  { model: "LambdaMART Re-ranker", ndcg: 0.81, mrr: 1.0, hitRate: 1.0 },
];

const TECH_STACK = [
  {
    name: "Go",
    role: "API Backend",
    reason: "Fast compilation, small binaries, and a concurrency model that handles high-throughput ranking calls without framework overhead.",
    icon: Zap,
  },
  {
    name: "Python FastAPI",
    role: "Ranking Service",
    reason: "The ML ecosystem lives in Python. FastAPI gives type-safe endpoints with Pydantic validation and sub-millisecond overhead.",
    icon: Cpu,
  },
  {
    name: "Supabase + pgvector",
    role: "Database & Vector Search",
    reason: "Postgres with pgvector replaces separate Elasticsearch and Redis instances. HNSW indexes give sub-50ms kNN queries at this scale.",
    icon: Database,
  },
  {
    name: "OpenAI Embeddings",
    role: "Representation Layer",
    reason: "text-embedding-3-small produces 1536-dim vectors from movie metadata. One API call per movie, stored once, queried forever.",
    icon: Layers,
  },
  {
    name: "Next.js",
    role: "Frontend",
    reason: "Server components for SEO-critical pages, client components for interactivity. Deployed on Vercel with edge caching.",
    icon: Globe,
  },
  {
    name: "LightGBM",
    role: "Learned Ranking",
    reason: "LambdaMART objective directly optimizes NDCG. Trains in seconds on interaction data, inference in microseconds.",
    icon: BarChart3,
  },
];

export default async function HowItWorksPage() {
  const seedMovies = await fetchSeedMovies();

  return (
    <article className="min-h-screen font-serif">
      {/* ── Hero ─────────────────────────────────────────────────── */}
      <header className="px-4 pb-20 pt-32">
        <div className="mx-auto max-w-3xl text-center">
          <p className="eyebrow mb-6 text-primary">Engineering deep dive</p>
          <h1 className="mb-6 font-heading text-3xl font-semibold uppercase leading-[1.1] tracking-tight sm:text-5xl lg:text-6xl">
            How CineMatch builds recommendations
          </h1>
          <p className="mx-auto max-w-xl text-lg leading-relaxed text-muted-foreground">
            A two-stage pipeline that combines vector similarity search with a
            learned ranking model to surface titles you will actually want to
            watch.
          </p>
        </div>
      </header>

      {/* ── Section 1: Pipeline Overview ─────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-5xl">
          <SectionHeading
            number="01"
            title="How recommendations work"
            subtitle="The two-stage pipeline"
          />
          <p className="mb-12 max-w-2xl leading-relaxed text-muted-foreground">
            Every recommendation request flows through two stages. First, we
            cast a wide net using vector search to find movies that are
            semantically close to the user&apos;s taste. Then, a scoring model
            re-ranks those candidates using richer signals to surface the
            best results.
          </p>
          <PipelineDiagram />
        </div>
      </SectionReveal>

      {/* ── Section 2: Retrieval ──────────────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-4xl">
          <SectionHeading
            number="02"
            title="The retrieval stage"
            subtitle="Vector search with pgvector"
          />
          <div className="mb-12 grid gap-12 md:grid-cols-2">
            <div className="space-y-5">
              <p className="leading-relaxed text-muted-foreground">
                Every movie is converted into a 1536-dimensional embedding using
                OpenAI&apos;s text-embedding-3-small model. The input
                combines the movie&apos;s plot summary, genres, release year,
                and key metadata into a single dense vector that captures
                its semantic identity.
              </p>
              <p className="leading-relaxed text-muted-foreground">
                User preferences are encoded the same way, built from the
                embeddings of movies they have liked and watched, weighted by
                recency.
              </p>
              <p className="leading-relaxed text-muted-foreground">
                Finding candidates is a nearest-neighbor search: we use
                pgvector&apos;s HNSW index to find the 50 movies with the
                highest cosine similarity to the user&apos;s embedding. This
                runs in under 50ms, even across the full catalog.
              </p>
            </div>
            <div className="space-y-4">
              <div className="border border-border bg-wash p-5">
                <p className="eyebrow mb-3 text-primary">Embedding space</p>
                <div className="space-y-3 text-sm">
                  <div className="flex justify-between text-muted-foreground">
                    <span>Dimensions</span>
                    <span className="font-mono text-foreground">1,536</span>
                  </div>
                  <div className="h-px w-full bg-border" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Distance metric</span>
                    <span className="font-mono text-foreground">
                      Cosine similarity
                    </span>
                  </div>
                  <div className="h-px w-full bg-border" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Index type</span>
                    <span className="font-mono text-foreground">
                      HNSW (m=16, ef=64)
                    </span>
                  </div>
                  <div className="h-px w-full bg-border" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Candidates returned</span>
                    <span className="font-mono text-foreground">50</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Interactive demo */}
          <div className="mt-16">
            <h3 className="mb-2 font-heading text-2xl font-semibold uppercase tracking-tight">
              Try it yourself
            </h3>
            <p className="mb-6 max-w-lg text-muted-foreground">
              Pick any title below to see its 5 nearest neighbors in
              embedding space. This calls the real pgvector index with live
              data.
            </p>
            <SimilarMoviesDemo seedMovies={seedMovies} />
          </div>
        </div>
      </SectionReveal>

      {/* ── Section 3: Ranking ───────────────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-4xl">
          <SectionHeading
            number="03"
            title="The ranking stage"
            subtitle="Multi-signal re-ranking"
          />
          <p className="mb-12 max-w-2xl leading-relaxed text-muted-foreground">
            Raw similarity is not enough. A movie can be close in embedding
            space but poorly rated, or popular but not to the user&apos;s
            taste. The ranking stage combines multiple signals into a single
            score that balances relevance, quality, and diversity.
          </p>

          {/* Feature weights */}
          <div className="mb-12 border border-border bg-wash p-6 sm:p-8">
            <p className="eyebrow mb-6 text-primary">Scoring weights</p>
            <div className="space-y-5">
              {FEATURE_WEIGHTS.map((f) => (
                <div key={f.name}>
                  <div className="mb-2 flex items-baseline justify-between">
                    <span className="text-sm font-medium text-foreground">
                      {f.name}
                    </span>
                    <span className="font-mono text-sm text-primary">
                      {(f.weight * 100).toFixed(0)}%
                    </span>
                  </div>
                  <div className="h-1.5 w-full bg-muted">
                    <div
                      className="h-full bg-primary transition-all duration-1000 ease-out"
                      style={{ width: `${f.weight * 100}%` }}
                    />
                  </div>
                  <p className="mt-1.5 text-xs text-muted-foreground">
                    {f.description}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className="border-l-2 border-primary/40 pl-6">
            <p className="text-sm leading-relaxed text-muted-foreground">
              <span className="font-medium text-foreground">
                Learned re-ranker:
              </span>{" "}
              The weights above are the transparent linear baseline.
              Production serves a LambdaMART model (LightGBM) that directly
              optimizes NDCG and learns non-linear preferences the handcrafted
              weights cannot capture, such as a vote-average sweet spot or an
              era preference. It leads the offline eval below. The ranker
              service supports both models and routes between them per request.
            </p>
          </div>
        </div>
      </SectionReveal>

      {/* ── Section 4: Evaluation ────────────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-4xl">
          <SectionHeading
            number="04"
            title="Evaluation"
            subtitle="Measuring recommendation quality"
          />

          {/* Metric definitions */}
          <div className="mb-12 grid gap-6 sm:grid-cols-3">
            <MetricCard
              name="NDCG@10"
              definition="Measures whether the most relevant movies appear at the top of the list, penalizing good recommendations buried at position 8 more than position 2."
            />
            <MetricCard
              name="MRR"
              definition="How quickly a user finds something they want. It measures the average rank of the first relevant result across all users."
            />
            <MetricCard
              name="Hit Rate@10"
              definition="The simplest test: does the top-10 list contain at least one movie the user would actually enjoy?"
            />
          </div>

          {/* Results table */}
          <div className="overflow-x-auto border border-border">
            <table className="w-full min-w-[28rem] text-sm">
              <thead>
                <tr className="bg-wash">
                  <th className="eyebrow px-5 py-3 text-left text-muted-foreground">
                    Model
                  </th>
                  <th className="eyebrow px-5 py-3 text-right text-muted-foreground">
                    NDCG@10
                  </th>
                  <th className="eyebrow px-5 py-3 text-right text-muted-foreground">
                    MRR
                  </th>
                  <th className="eyebrow px-5 py-3 text-right text-muted-foreground">
                    Hit Rate
                  </th>
                </tr>
              </thead>
              <tbody>
                {EVAL_RESULTS.map((r, i) => (
                  <tr
                    key={r.model}
                    className={
                      i === EVAL_RESULTS.length - 1
                        ? "bg-accent"
                        : "border-t border-border"
                    }
                  >
                    <td className="px-5 py-3.5 font-medium text-foreground">
                      {r.model}
                      {i === EVAL_RESULTS.length - 1 && (
                        <span className="ml-2 font-mono text-[10px] uppercase tracking-wider text-primary">
                          Current
                        </span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 text-right font-mono">
                      <ScoreCell
                        value={r.ndcg}
                        best={i === EVAL_RESULTS.length - 1}
                      />
                    </td>
                    <td className="px-5 py-3.5 text-right font-mono">
                      <ScoreCell
                        value={r.mrr}
                        best={i === EVAL_RESULTS.length - 1}
                      />
                    </td>
                    <td className="px-5 py-3.5 text-right font-mono">
                      <ScoreCell
                        value={r.hitRate}
                        best={i === EVAL_RESULTS.length - 1}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-4 text-xs text-muted-foreground">
            Evaluated on 40 held-out synthetic users with 1,695 interactions
            across 494 movies. Synthetic users have non-linear taste profiles
            (favourite era, vote-average sweet spot, genre-dependent recency)
            with Gaussian noise to simulate realistic behavior.
          </p>
        </div>
      </SectionReveal>

      {/* ── Section 5: Cold Start ────────────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-3xl">
          <SectionHeading
            number="05"
            title="Cold start"
            subtitle="What happens for new users"
          />
          <div className="space-y-6 leading-relaxed text-muted-foreground">
            <p>
              A new user has no interaction history, which means no user
              embedding and no signal for the ranking model. Rather than
              showing nothing, the pipeline falls back gracefully through
              three tiers:
            </p>
          </div>

          <div className="mt-10 space-y-6">
            <ColdStartTier
              stage="0 interactions"
              label="Popularity Fallback"
              description="The system returns the most popular, highest-rated movies across all genres. No personalization, but the recommendations are still high quality."
              blend="100% popular"
            />
            <ColdStartTier
              stage="1-5 interactions"
              label="Content-Based Filtering"
              description="After a few likes or watches, the system builds a preliminary user embedding from the movies' own embeddings. Cosine similarity retrieval begins, blended with popular results."
              blend="60% popular, 40% personalized"
            />
            <ColdStartTier
              stage="6+ interactions"
              label="Full Pipeline"
              description="With enough signal, the two-stage pipeline activates fully. The user embedding stabilizes, and the ranking model has enough context to re-score candidates meaningfully."
              blend="100% personalized"
            />
          </div>
        </div>
      </SectionReveal>

      {/* ── Section 6: Tech Stack ────────────────────────────────── */}
      <SectionReveal className="border-t border-border px-4 py-20">
        <div className="mx-auto max-w-4xl">
          <SectionHeading number="06" title="Tech stack" subtitle="Built with" />
          <div className="mb-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {TECH_STACK.map((t) => (
              <div
                key={t.name}
                className="border border-border bg-card p-5 transition-colors duration-200 hover:border-primary"
              >
                <div className="mb-3 flex items-center gap-3">
                  <t.icon className="size-4 text-primary" strokeWidth={1.5} />
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {t.name}
                    </p>
                    <p className="text-[11px] text-muted-foreground">
                      {t.role}
                    </p>
                  </div>
                </div>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t.reason}
                </p>
              </div>
            ))}
          </div>

          <div className="flex justify-center">
            <Link
              href="https://github.com/QHarshil/CineMatch"
              target="_blank"
              rel="noopener noreferrer"
              className="eyebrow inline-flex items-center gap-2 border border-border px-6 py-3 text-foreground transition-colors duration-200 hover:border-primary hover:text-primary"
            >
              <ExternalLink className="size-4" strokeWidth={1.5} />
              View source on GitHub
            </Link>
          </div>
        </div>
      </SectionReveal>

      {/* Spacer for footer breathing room */}
      <div className="h-20" />

      {/* Inline keyframes for the demo animation */}
      <style>{`
        @keyframes fadeSlideIn {
          from {
            opacity: 0;
            transform: translateX(-12px);
          }
          to {
            opacity: 1;
            transform: translateX(0);
          }
        }
      `}</style>
    </article>
  );
}

/* ── Sub-components ──────────────────────────────────────────────── */

function SectionHeading({
  number,
  title,
  subtitle,
}: {
  number: string;
  title: string;
  subtitle: string;
}) {
  return (
    <div className="mb-10">
      <p className="mb-3 font-mono text-xs text-primary/50">{number}</p>
      <h2 className="mb-2 font-heading text-3xl font-semibold uppercase tracking-tight sm:text-4xl">
        {title}
      </h2>
      <p className="eyebrow text-muted-foreground">{subtitle}</p>
    </div>
  );
}

function MetricCard({
  name,
  definition,
}: {
  name: string;
  definition: string;
}) {
  return (
    <div className="border border-border bg-card p-5">
      <p className="mb-2 font-mono text-sm text-primary">{name}</p>
      <p className="text-xs leading-relaxed text-muted-foreground">
        {definition}
      </p>
    </div>
  );
}

function ScoreCell({ value, best }: { value: number; best: boolean }) {
  return (
    <span className={best ? "font-medium text-primary" : "text-muted-foreground"}>
      {value.toFixed(2)}
    </span>
  );
}

function ColdStartTier({
  stage,
  label,
  description,
  blend,
}: {
  stage: string;
  label: string;
  description: string;
  blend: string;
}) {
  return (
    <div className="flex gap-5">
      <div className="flex shrink-0 flex-col items-center">
        <div className="size-3 border border-primary bg-primary/15" />
        <div className="w-px flex-1 bg-border" />
      </div>
      <div className="pb-8">
        <p className="mb-1 font-mono text-xs text-primary">{stage}</p>
        <p className="mb-2 font-heading text-lg font-semibold">{label}</p>
        <p className="mb-2 text-sm leading-relaxed text-muted-foreground">
          {description}
        </p>
        <p className="font-mono text-xs text-muted-foreground/70">{blend}</p>
      </div>
    </div>
  );
}
