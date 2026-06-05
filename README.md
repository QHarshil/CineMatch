# CineMatch

A movie recommendation engine using two-stage retrieve-then-rank architecture. Users rate movies, the system builds a taste profile, and recommendations improve with each interaction.

Live at [cinematch.harshilc.com](https://cinematch.harshilc.com)

## How it works

The recommendation pipeline runs in two stages:

1. **Retrieval:** A user's taste is encoded as a 1536-dimensional embedding. Supabase pgvector finds the 50 nearest movies by cosine similarity using an HNSW index.
2. **Ranking:** A Python microservice re-scores those 50 candidates using a weighted feature model (or a trained LambdaMART model), combining similarity, quality, popularity, and genre overlap. The top 20 come back to the frontend.

Cold-start users (no interactions yet) get popularity-ranked movies. If the ranker is down, candidates return in their original similarity order. If Supabase is unreachable, an in-memory cache of popular movies keeps the site functional.

## Architecture

```
Browser
  |
  |  HTTPS
  v
Next.js Frontend (Vercel)
  |  - Supabase Auth (magic link, no passwords)
  |  - Direct Supabase reads for browsing (RLS-protected)
  |  - Calls Go API for search + recommendations
  |
  |  REST
  v
Go API (Cloud Run)
  |  - Chi router, 9-layer middleware stack
  |  - Per-endpoint rate limiting (10-30 req/min by route)
  |  - JWT auth via Supabase secret
  |
  +-------> Supabase Postgres
  |           - pgvector HNSW indexes (1536-dim, cosine)
  |           - RLS on all tables, service key server-side only
  |           - match_movies() RPC for kNN retrieval
  |
  +-------> Python Ranker (Cloud Run)
              - FastAPI, POST /rank
              - feature-linear-v1: explicit weighted formula
              - lambdamart-v1: LightGBM learned model
```

## Tech stack

| Layer | Choice | Why |
|-------|--------|-----|
| Frontend | Next.js 16, TypeScript strict, Tailwind, shadcn/ui | App Router for SSR movie pages, client components for interactive bits |
| API | Go 1.22, Chi router | Fast compilation, small binary, Chi's composable middleware |
| Database | Supabase Postgres + pgvector | Managed Postgres with vector search built in, no separate search infra |
| Embeddings | OpenAI text-embedding-3-small (1536-dim) | Good quality-to-cost ratio, single API call per movie |
| Ranker | Python 3.12, FastAPI, LightGBM | Python for ML flexibility, FastAPI for async, LightGBM for LambdaMART |
| Auth | Supabase magic link | Passwordless, no credential storage |
| Hosting | Vercel + Google Cloud Run | Vercel for the frontend CDN; Cloud Run for the Go API and ranker containers (scale-to-zero, request-billed) |

## Getting started

You need Go 1.22+, Node.js 20+, Python 3.12+, a [Supabase](https://supabase.com) project, a [TMDB](https://www.themoviedb.org/settings/api) API key, and an [OpenAI](https://platform.openai.com) API key.

```bash
git clone https://github.com/QHarshil/CineMatch.git
cd CineMatch
cp .env.example .env          # fill in credentials
```

Then, in three terminals:

```bash
# Terminal 1: Go API
cd backend && go run .

# Terminal 2: Python ranker
cd ranker && pip install -r requirements.txt && uvicorn main:app --port 8000

# Terminal 3: Next.js frontend
cd frontend && npm install && npm run dev
```

The frontend runs on `localhost:3000`, the API on `localhost:8080`, the ranker on `localhost:8000`.

## Environment variables

Copy `.env.example` to `.env` and fill in:

| Variable | Used by | Description |
|----------|---------|-------------|
| `SUPABASE_URL` | backend, scripts | Supabase project URL |
| `SUPABASE_SECRET_KEY` | backend, scripts | Service-role key (never in frontend) |
| `NEXT_PUBLIC_SUPABASE_URL` | frontend | Same Supabase URL, exposed to browser |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | frontend | Publishable anon key (RLS restricts access) |
| `JWT_SECRET` | backend | Supabase JWT secret for token verification |
| `TMDB_READ_ACCESS_TOKEN` | backend, scripts | TMDB v4 Bearer token |
| `OPENAI_API_KEY` | backend, scripts | For embedding generation |
| `ALLOWED_ORIGINS` | backend | Comma-separated CORS origins |
| `APP_PORT` | backend | HTTP listen port (default `8080`) |
| `RANKER_URL` | backend | Python ranker URL (default `http://localhost:8000`) |

## Repo structure

```
CineMatch/
  backend/     Go API server, middleware, Supabase client
  frontend/    Next.js app, pages, components, design system
  ranker/      Python ranking microservice (FastAPI)
  eval/        Offline evaluation pipeline and synthetic data generation
  scripts/     TMDB seeder and data backfill scripts
```

Each subdirectory has its own README with setup instructions and API contracts.

## Evaluation results

Offline eval on synthetic data (200 users across 8 taste profiles, 8,871 interactions; 40 users / 1,695 interactions held out for test). Every number is produced by `eval/eval_rankers.py`:

| Model | NDCG@10 | MRR | Hit Rate@10 |
|-------|---------|-----|-------------|
| Popularity baseline | 0.716 | 0.875 | 1.00 |
| Vector retrieval only | 0.798 | 0.938 | 1.00 |
| Two-stage (linear ranker) | 0.795 | 0.950 | 1.00 |
| Two-stage (LambdaMART) | 0.814 | 1.000 | 1.00 |

The synthetic users have non-linear preferences (a favourite release era, a vote-average sweet spot, recency that depends on genre) that a fixed-weight linear formula cannot represent. LambdaMART captures them and leads on NDCG@10: +14% over the popularity baseline, and ahead of both retrieval-only and the linear re-ranker. Stage-2 re-ranking runs in ~0.9 ms p95 (`eval/benchmark_latency.py`). See [eval/README.md](eval/README.md) for the methodology.
