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
Go API (Railway)
  |  - Chi router, 9-layer middleware stack
  |  - Per-endpoint rate limiting (10-30 req/min by route)
  |  - JWT auth via Supabase secret
  |
  +-------> Supabase Postgres
  |           - pgvector HNSW indexes (1536-dim, cosine)
  |           - RLS on all tables, service key server-side only
  |           - match_movies() RPC for kNN retrieval
  |
  +-------> Python Ranker (Railway, internal only)
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
| Hosting | Vercel + Railway | Vercel for frontend CDN, Railway for backend containers |

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

Offline eval on synthetic data (200 users, 8 taste profiles, 9607 interactions):

| Model | NDCG@10 | MRR | Hit Rate@10 |
|-------|---------|-----|-------------|
| Popularity baseline | 0.62 | 0.71 | 0.85 |
| Vector retrieval only | 0.76 | 0.89 | 0.95 |
| Two-stage (linear ranker) | 0.86 | 1.00 | 1.00 |
| Two-stage (LambdaMART) | 0.71 | 0.86 | 1.00 |

The linear ranker outperforms LambdaMART on synthetic data because the synthetic generation process mirrors the linear formula. On real user data with messier preferences, the learned model should close that gap. See [eval/README.md](eval/README.md) for details.
