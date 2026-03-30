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
  title: "How It Works | CineMatch",
  description:
    "A technical deep-dive into how CineMatch builds personalized movie recommendations using vector search and learned ranking.",
};

const MOVIE_FIELDS =
  "id, title, poster_path" as const;

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
  { model: "Popularity Baseline", ndcg: 0.62, mrr: 0.71, hitRate: 0.85 },
  { model: "Vector Retrieval Only", ndcg: 0.76, mrr: 0.89, hitRate: 0.95 },
  { model: "Two-Stage Pipeline", ndcg: 0.86, mrr: 1.00, hitRate: 1.00 },
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
    <article className="min-h-screen">
      {/* ── Hero ─────────────────────────────────────────────────── */}
      <header className="pt-32 pb-20 px-4">
        <div className="max-w-3xl mx-auto text-center">
          <p className="text-xs tracking-[0.3em] uppercase text-gold mb-6">
            Engineering Deep Dive
          </p>
          <h1 className="font-heading text-4xl sm:text-5xl lg:text-6xl font-bold leading-[1.1] mb-6">
            How CineMatch Builds Recommendations
          </h1>
          <p className="text-lg text-muted-foreground leading-relaxed max-w-xl mx-auto">
            A two-stage pipeline that combines vector similarity search with a
            learned ranking model to surface movies you will actually want to
            watch.
          </p>
        </div>
      </header>

      {/* ── Section 1: Pipeline Overview ─────────────────────────── */}
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-5xl mx-auto">
          <SectionHeading
            number="01"
            title="How recommendations work"
            subtitle="The two-stage pipeline"
          />
          <p className="text-muted-foreground leading-relaxed max-w-2xl mb-12">
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
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-4xl mx-auto">
          <SectionHeading
            number="02"
            title="The retrieval stage"
            subtitle="Vector search with pgvector"
          />
          <div className="grid md:grid-cols-2 gap-12 mb-12">
            <div className="space-y-5">
              <p className="text-muted-foreground leading-relaxed">
                Every movie is converted into a 1536-dimensional embedding using
                OpenAI&apos;s text-embedding-3-small model. The input
                combines the movie&apos;s plot summary, genres, release year,
                and key metadata into a single dense vector that captures
                its semantic identity.
              </p>
              <p className="text-muted-foreground leading-relaxed">
                User preferences are encoded the same way, built from the
                embeddings of movies they have liked and watched, weighted by
                recency.
              </p>
              <p className="text-muted-foreground leading-relaxed">
                Finding candidates is a nearest-neighbor search: we use
                pgvector&apos;s HNSW index to find the 50 movies with the
                highest cosine similarity to the user&apos;s embedding. This
                runs in under 50ms, even across the full catalog.
              </p>
            </div>
            <div className="space-y-4">
              <div className="border border-border bg-surface/30 p-5">
                <p className="text-xs text-gold uppercase tracking-widest mb-3">
                  Embedding Space
                </p>
                <div className="space-y-3 text-sm">
                  <div className="flex justify-between text-muted-foreground">
                    <span>Dimensions</span>
                    <span className="text-foreground font-mono">1,536</span>
                  </div>
                  <div className="w-full h-px bg-border/50" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Distance metric</span>
                    <span className="text-foreground font-mono">
                      Cosine similarity
                    </span>
                  </div>
                  <div className="w-full h-px bg-border/50" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Index type</span>
                    <span className="text-foreground font-mono">
                      HNSW (m=16, ef=64)
                    </span>
                  </div>
                  <div className="w-full h-px bg-border/50" />
                  <div className="flex justify-between text-muted-foreground">
                    <span>Candidates returned</span>
                    <span className="text-foreground font-mono">50</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Interactive demo */}
          <div className="mt-16">
            <h3 className="font-heading text-2xl font-semibold mb-2">
              Try it yourself
            </h3>
            <p className="text-muted-foreground mb-6 max-w-lg">
              Pick any movie below to see its 5 nearest neighbors in
              embedding space. This calls the real pgvector index with live
              data.
            </p>
            <SimilarMoviesDemo seedMovies={seedMovies} />
          </div>
        </div>
      </SectionReveal>

      {/* ── Section 3: Ranking ───────────────────────────────────── */}
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-4xl mx-auto">
          <SectionHeading
            number="03"
            title="The ranking stage"
            subtitle="Multi-signal re-ranking"
          />
          <p className="text-muted-foreground leading-relaxed max-w-2xl mb-12">
            Raw similarity is not enough. A movie can be close in embedding
            space but poorly rated, or popular but not to the user&apos;s
            taste. The ranking stage combines multiple signals into a single
            score that balances relevance, quality, and diversity.
          </p>

          {/* Feature weights */}
          <div className="border border-border bg-surface/30 p-6 sm:p-8 mb-12">
            <p className="text-xs text-gold uppercase tracking-widest mb-6">
              Scoring Weights
            </p>
            <div className="space-y-5">
              {FEATURE_WEIGHTS.map((f) => (
                <div key={f.name}>
                  <div className="flex items-baseline justify-between mb-2">
                    <span className="text-sm font-medium text-foreground">
                      {f.name}
                    </span>
                    <span className="text-sm font-mono text-gold">
                      {(f.weight * 100).toFixed(0)}%
                    </span>
                  </div>
                  <div className="w-full h-1.5 bg-muted">
                    <div
                      className="h-full bg-gold transition-all duration-1000 ease-out"
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

          <div className="border-l-2 border-gold/30 pl-6">
            <p className="text-sm text-muted-foreground leading-relaxed">
              <span className="text-foreground font-medium">
                Upgrade path:
              </span>{" "}
              When sufficient real interaction data accumulates, the linear
              scorer is replaced by a LambdaMART model (LightGBM) that
              directly optimizes NDCG. The model learns non-linear
              feature interactions that handcrafted weights cannot capture,
              such as the relationship between genre preferences and
              popularity thresholds. The ranker service supports both
              models and routes between them per-request.
            </p>
          </div>
        </div>
      </SectionReveal>

      {/* ── Section 4: Evaluation ────────────────────────────────── */}
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-4xl mx-auto">
          <SectionHeading
            number="04"
            title="Evaluation"
            subtitle="Measuring recommendation quality"
          />

          {/* Metric definitions */}
          <div className="grid sm:grid-cols-3 gap-6 mb-12">
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
          <div className="border border-border overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-surface/50">
                  <th className="text-left px-5 py-3 text-xs text-muted-foreground uppercase tracking-wider font-medium">
                    Model
                  </th>
                  <th className="text-right px-5 py-3 text-xs text-muted-foreground uppercase tracking-wider font-medium">
                    NDCG@10
                  </th>
                  <th className="text-right px-5 py-3 text-xs text-muted-foreground uppercase tracking-wider font-medium">
                    MRR
                  </th>
                  <th className="text-right px-5 py-3 text-xs text-muted-foreground uppercase tracking-wider font-medium">
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
                        ? "bg-gold/5"
                        : "border-t border-border/30"
                    }
                  >
                    <td className="px-5 py-3.5 font-medium text-foreground">
                      {r.model}
                      {i === EVAL_RESULTS.length - 1 && (
                        <span className="ml-2 text-[10px] text-gold uppercase tracking-wider">
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
            Evaluated on 40 held-out synthetic users with 1,811 interactions
            across 494 movies. Synthetic users have genre-weighted taste
            profiles with Gaussian noise to simulate realistic behavior.
          </p>
        </div>
      </SectionReveal>

      {/* ── Section 5: Cold Start ────────────────────────────────── */}
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-3xl mx-auto">
          <SectionHeading
            number="05"
            title="Cold start"
            subtitle="What happens for new users"
          />
          <div className="space-y-6 text-muted-foreground leading-relaxed">
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
      <SectionReveal className="py-20 px-4 border-t border-border/30">
        <div className="max-w-4xl mx-auto">
          <SectionHeading
            number="06"
            title="Tech stack"
            subtitle="Built with"
          />
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5 mb-12">
            {TECH_STACK.map((t) => (
              <div
                key={t.name}
                className="border border-border bg-surface/30 p-5 transition-colors duration-200 hover:border-border/80"
              >
                <div className="flex items-center gap-3 mb-3">
                  <t.icon className="w-4 h-4 text-gold" strokeWidth={1.5} />
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {t.name}
                    </p>
                    <p className="text-[11px] text-muted-foreground">
                      {t.role}
                    </p>
                  </div>
                </div>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  {t.reason}
                </p>
              </div>
            ))}
          </div>

          <div className="flex justify-center">
            <Link
              href="https://github.com/harshil-c18/CineMatch"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 border border-border px-6 py-3 text-sm text-foreground hover:border-gold hover:text-gold transition-colors duration-200"
            >
              <ExternalLink className="w-4 h-4" strokeWidth={1.5} />
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
      <p className="text-xs font-mono text-gold/60 mb-3">{number}</p>
      <h2 className="font-heading text-3xl sm:text-4xl font-bold mb-2">
        {title}
      </h2>
      <p className="text-sm text-muted-foreground uppercase tracking-widest">
        {subtitle}
      </p>
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
    <div className="border border-border bg-surface/30 p-5">
      <p className="font-mono text-sm text-gold mb-2">{name}</p>
      <p className="text-xs text-muted-foreground leading-relaxed">
        {definition}
      </p>
    </div>
  );
}

function ScoreCell({ value, best }: { value: number; best: boolean }) {
  return (
    <span className={best ? "text-gold font-medium" : "text-muted-foreground"}>
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
      <div className="flex flex-col items-center shrink-0">
        <div className="w-3 h-3 border border-gold bg-gold/20" />
        <div className="w-px flex-1 bg-border/30" />
      </div>
      <div className="pb-8">
        <p className="text-xs text-gold font-mono mb-1">{stage}</p>
        <p className="font-heading text-lg font-semibold mb-2">{label}</p>
        <p className="text-sm text-muted-foreground leading-relaxed mb-2">
          {description}
        </p>
        <p className="text-xs text-muted-foreground/60 font-mono">{blend}</p>
      </div>
    </div>
  );
}
