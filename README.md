# CineMatch

A film and TV recommender with a grounded AI assistant. Describe a mood or name a title you loved, and an agent searches the catalog, reads your taste, and calls a two-stage recommender. Every title it recommends comes from a tool result, and every step is streamed to the page and written to an audit log.

Live at [cinematch.harshilc.com](https://cinematch.harshilc.com)

## What it does

- **Ask** (`/assistant`): a tool-calling agent over the catalog. It streams each tool call as it runs, validates its picks against what the tools returned, and falls back to plain search when the model is unavailable or over budget. Guests can try it without an email.
- **Search by meaning** (`/browse?q=`): hybrid retrieval. pgvector similarity, Postgres full text, and trigram title matching are fused with reciprocal rank fusion, so "a crew planning the perfect robbery" and "intersteller" both work.
- **For You** (`/for-you`): a two-stage recommender. pgvector retrieves 50 candidates near your taste vector, then a LightGBM LambdaMART model re-ranks them. Each pick shows the liked title it is closest to and the ranker's SHAP factors.

## Architecture

```
Browser
  |
  v
Next.js 16 frontend (Vercel)            React 19, TypeScript strict, Tailwind v4
  |  Supabase Auth: magic link or guest session
  |  Lenis + GSAP motion, Vanta hero, reduced-motion safe
  |
  |  REST + server-sent events
  v
Go API (Cloud Run)                      Chi, 9-layer middleware, per-route rate limits
  |
  |-- /discover  -> search.Service -> embed.Budgeted (OpenAI, LRU + daily cap)
  |                                -> search_titles_hybrid RPC (vector + full text + trigram, RRF)
  |
  |-- /assistant -> assistant.Agent (tool-calling loop, grounding by ref, output guard)
  |                 |-> llm.Client: any OpenAI-compatible API (Ollama, Gemini, Groq, OpenAI)
  |                 |-> tools: search_catalog, find_similar, get_taste_profile,
  |                 |          get_recommendations, present_picks
  |                 '-> assistant_runs audit log + per-user, guest, and global budgets
  |
  |-- /recommend -> pgvector kNN (match_movies) -> Python ranker (LambdaMART + SHAP)
  |
  +--> Supabase Postgres: pgvector HNSW, RLS on every table
  '--> Python ranker (Cloud Run): FastAPI, LightGBM
```

Everything degrades instead of failing: no model or a rate limit returns search results, no query vector runs keyword search, a ranker outage returns similarity order, and a database outage serves an in-memory cache of popular titles.

## The AI layer

The agent plans with a language model but answers only from the catalog.

- **Read-only tools.** The agent can look things up but cannot change a profile. Likes happen only when the person clicks.
- **Grounding by ref.** Every title a tool returns gets a short ref such as `t3`, and `present_picks` accepts only refs the agent has seen. Anything else is rejected on the server and counted in the audit log. Small models copy short refs reliably where they garble UUIDs.
- **Small-model fixes in code.**
  - Genre words are mapped across TMDB's film and series vocabularies.
  - Quoted numbers are accepted.
  - A title hidden by a filter is surfaced separately as a seed.
  - A prose answer after tool calls is sent back once to be finished through `present_picks`.
- **Output guard.** Replies that repeat the instructions (tool names, headings, any eight-word run) are replaced before they are sent. The agent eval found this failure on qwen3:8b.
- **Budgets that fail closed.**
  - A per-user daily limit, and a smaller one for guests.
  - A global run and token cap, and an embedding call cap.
  - If usage cannot be read, no model call is made.
- **Privacy.** Emails are stored as SHA-256 hashes, prompts are stored as hashes, and emails and phone numbers are masked in logged tool arguments.

## Evaluation

Every number on the site comes from one of these scripts.

### Retrieval

`eval/ai/search.eval.ts` covers 15 title lookups (including typos) and 18 paraphrased descriptions that avoid genre words, over 1,843 titles. A description result is relevant when its TMDB genres match the target, a label none of the rankers read.

| Retrieval | Title MRR@10 | Title Hit@1 | Description P@10 | Description nDCG@10 |
|-----------|--------------|-------------|------------------|---------------------|
| Keyword only | 1.000 | 1.000 | 0.011 | 0.020 |
| Semantic only | 0.967 | 0.933 | 0.739 | 0.726 |
| Hybrid (production) | 1.000 | 1.000 | 0.739 | 0.726 |

A random ranking scores 0.152 P@10. Keyword search finds titles but not moods; embeddings find moods but miss some typos; hybrid matches the better of the two on both.

### Agent

`eval/ai/assistant.eval.ts` runs 23 cases through the real API as a dedicated eval user, parsing the stream with the frontend's own client.

| Category | Passed |
|----------|--------|
| Hard constraints (series, years, runtime, language, genre) | 10/10 |
| Like a named title | 4/4 |
| From personal taste | 2/2 |
| Vague request asks a question | 2/2 |
| Off-topic request declines | 2/2 |
| Prompt injection does not leak instructions | 3/3 |

These results are from qwen3:8b on Ollama:
- Pick-level constraint satisfaction was 66/66, and no ungrounded picks were shown.
- Runs averaged 1.57 tool calls and 6,274 tokens.
- Latency was 10.8 s at p50 and 19.3 s at p95, on a laptop.
- Before the output guard, one injection case leaked part of the instructions (22/23).

### Ranker

Offline eval on synthetic data (200 users across 8 taste profiles, 8,871 interactions; 40 users / 1,695 interactions held out for test). Every number is produced by `eval/eval_rankers.py`:

| Model | NDCG@10 | MRR | Hit Rate@10 |
|-------|---------|-----|-------------|
| Popularity baseline | 0.716 | 0.875 | 1.00 |
| Vector retrieval only | 0.798 | 0.938 | 1.00 |
| Two-stage (linear ranker) | 0.795 | 0.950 | 1.00 |
| Two-stage (LambdaMART) | 0.814 | 1.000 | 1.00 |

The synthetic users have non-linear preferences (a favourite release era, a vote-average sweet spot, recency that depends on genre) that a fixed-weight linear formula cannot represent. LambdaMART captures them and leads on NDCG@10: +14% over the popularity baseline, and ahead of both retrieval-only and the linear re-ranker. Stage-2 re-ranking runs in ~0.9 ms p95 (`eval/benchmark_latency.py`). See [eval/README.md](eval/README.md) for the methodology.

## Getting started

You need Go 1.22+, Node.js 24+, Python 3.12+, a [Supabase](https://supabase.com) project, and a [TMDB](https://www.themoviedb.org/settings/api) token. An [OpenAI](https://platform.openai.com) key enables embedding search; without it, search runs keyword-only. For the assistant, install [Ollama](https://ollama.com) and pull a tool-calling model:

```bash
brew install ollama && ollama serve
ollama pull qwen3:8b
```

```bash
git clone https://github.com/QHarshil/CineMatch.git
cd CineMatch
cp .env.example .env          # fill in credentials; LLM_* already point at local Ollama
```

Apply the SQL in `migrations/` to your Supabase project, then, in three terminals:

```bash
# Terminal 1: Go API
cd backend && go run .

# Terminal 2: Python ranker
cd ranker && pip install -r requirements.txt && uvicorn main:app --port 8000

# Terminal 3: Next.js frontend
cd frontend && npm install && npm run dev
```

The frontend runs on `localhost:3000`, the API on `localhost:8080`, the ranker on `localhost:8000`.

To run the evals against the local API:

```bash
cd eval/ai && npm ci
npm run eval:search
ASSISTANT_USER_DAILY_RUNS=1000 npm run eval:assistant   # set on the backend you run it against
```

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
| `OPENAI_API_KEY` | backend, scripts | Catalog embeddings (seeder) and query embeddings for natural-language search |
| `EMBED_DAILY_LIMIT` | backend | Cap on upstream query-embedding calls per instance per day (default `5000`) |
| `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY` | backend | Assistant model on any OpenAI-compatible API (Ollama locally, a hosted free tier in production) |
| `LLM_REASONING_EFFORT` | backend | `none` turns off thinking on reasoning models |
| `ASSISTANT_USER_DAILY_RUNS`, `ASSISTANT_GUEST_DAILY_RUNS`, `ASSISTANT_DAILY_RUNS`, `ASSISTANT_DAILY_TOKENS` | backend | Per-user, per-guest, and global daily caps on assistant use |
| `ALLOWED_ORIGINS` | backend | Comma-separated CORS origins |
| `APP_PORT` | backend | HTTP listen port (default `8080`) |
| `RANKER_URL` | backend | Python ranker URL (default `http://localhost:8000`) |
| `OMDB_API_KEY` | backend | OMDb key for IMDb/Rotten Tomatoes ratings (optional; ratings hidden if unset) |

## Repo structure

```
CineMatch/
  backend/     Go API: hybrid search, the assistant agent, the recommender pipeline, middleware
  frontend/    Next.js app: assistant, browse, For You, How it works, motion system
  ranker/      Python ranking service (FastAPI, LightGBM LambdaMART with SHAP factors)
  eval/        Ranker eval pipeline (Python) and retrieval and agent evals (eval/ai, TypeScript)
  scripts/     TMDB seeder (Go) and detail backfill (TypeScript)
  migrations/  Supabase SQL: hybrid search, audit log, explanations, guest users
  deploy/      Cloud Run deploy scripts and the billing kill switch
```

Each subdirectory has its own README with setup instructions and API contracts.

## Tech stack

| Layer | Choice | Why |
|-------|--------|-----|
| Frontend | Next.js 16, React 19, TypeScript strict, Tailwind v4, shadcn/ui | Server components for catalog pages, client components for streaming UI |
| Motion | Lenis, GSAP (ScrollTrigger, SplitText, ScrambleText), Vanta NET, React Bits patterns | Smooth scroll synced to scroll-driven animation; every effect off under reduced motion |
| API | Go, Chi | Small binary, composable middleware, straightforward streaming |
| Assistant model | Any OpenAI-compatible chat API | Ollama locally for free; a hosted free tier in production |
| Search | Supabase Postgres, pgvector HNSW, full text, pg_trgm | One database for vectors, text, and data; fused with RRF |
| Embeddings | OpenAI text-embedding-3-small (1536-dim) | The model the catalog was embedded with |
| Ranker | Python 3.12, FastAPI, LightGBM | LambdaMART optimizes NDCG directly; SHAP explains each pick |
| Evals | Python (ranker), TypeScript (retrieval and agent) | Agent evals reuse the frontend's stream parser |
| Hosting | Vercel + Google Cloud Run | CDN frontend; scale-to-zero containers for the API and ranker |
| CI | GitHub Actions | Go vet and race tests, pytest, Vitest, tsc, lint, and build on every push |
