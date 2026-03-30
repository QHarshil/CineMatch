# CineMatch

A movie recommendation engine that combines vector similarity search with a learned ranking model. You rate movies, CineMatch learns your taste, and surfaces what you actually want to watch next.

Live at [cinematch.harshilc.com](https://cinematch.harshilc.com)

## Architecture

```
Browser (cinematch.harshilc.com)
         |
         | HTTPS
         v
Next.js 14 Frontend  (Vercel)
  - Supabase Auth (magic link)
  - Direct Supabase reads for movie data (RLS)
  - Calls Go API for search + recommendations
         |
         | REST
         v
Go API Backend  (Railway)
  - Chi router + CORS / rate-limit / logging middleware
  - Supabase service key for server-side writes
  - OpenAI API for embedding generation
  - Calls Python ranker for Stage-2 re-scoring
         |              |
    Supabase       Python Ranker  (Railway, internal)
   Postgres          FastAPI + LambdaMART/MLP
   pgvector          POST /rank
   HNSW indexes
   RLS on all tables
```

### Two-stage recommendation pipeline

1. **Retrieval (Go):** user embedding -> `match_movies()` RPC -> top 50 candidates by cosine similarity (pgvector HNSW)
2. **Ranking (Python):** candidates + user features -> LambdaMART/MLP re-scores -> top 20 returned to frontend

## Tech stack

| Layer | Technology |
|-------|------------|
| Frontend | Next.js 14 App Router, TypeScript strict, Tailwind CSS, shadcn/ui |
| Backend | Go 1.22, Chi router |
| Database | Supabase Postgres + pgvector (1536-dim HNSW indexes) |
| Embeddings | OpenAI text-embedding-3-small |
| Ranker | Python 3.12, FastAPI, LambdaMART / lightweight MLP |
| Eval | Python: MRR, NDCG@k, Hit Rate offline pipeline |
| Hosting | Vercel (frontend), Railway (Go + Python) |
| Auth | Supabase magic link (no passwords stored) |

## Repo structure

```
CineMatch/
├── backend/      # Go API (Chi router, Supabase client, handlers)
├── frontend/     # Next.js 14 app
├── ranker/       # Python FastAPI ranking microservice
├── eval/         # Offline evaluation pipeline (MRR, NDCG@k)
└── scripts/      # TMDB seeder (fetches movies + generates embeddings)
```

Each subdirectory has its own README with setup instructions and API contracts.

## Running locally

### Prerequisites

- Go 1.22+
- Node.js 20+
- Python 3.12+
- A Supabase project (see environment variables below)
- TMDB API account (free)
- OpenAI API key

### Backend

```bash
cd backend
cp ../.env.example ../.env   # fill in your credentials
go run .
# Starts on http://localhost:8080
```

### Frontend

```bash
cd frontend
npm install
npm run dev
# Starts on http://localhost:3000
```

### Python ranker

```bash
cd ranker
pip install -r requirements.txt
uvicorn main:app --reload --port 8001
```

## Environment variables

Copy `.env.example` to `.env` and fill in:

| Variable | Used by | Description |
|----------|---------|-------------|
| `SUPABASE_URL` | backend | Supabase project URL |
| `SUPABASE_SECRET_KEY` | backend only | Service-role key — never in frontend |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | frontend only | Publishable key |
| `TMDB_READ_ACCESS_TOKEN` | backend / scripts | TMDB Bearer token |
| `OPENAI_API_KEY` | backend | For embedding generation |
| `ALLOWED_ORIGINS` | backend | Comma-separated CORS origins |
| `RATE_LIMIT_RPM` | backend | Requests/min per IP (default 60) |
| `APP_PORT` | backend | HTTP listen port (default 8080) |

## Deploying

- **Frontend:** connect the `frontend/` directory to a Vercel project. Set `NEXT_PUBLIC_SUPABASE_ANON_KEY` in Vercel environment settings.
- **Backend:** connect `backend/` to a Railway service. The Dockerfile is included. Set all non-`NEXT_PUBLIC_` variables in the Railway dashboard.
- **Ranker:** connect `ranker/` to a separate Railway service (internal networking only — not exposed publicly).
- **DNS:** point `cinematch.harshilc.com` to the Vercel deployment.

## Evaluation

```bash
cd eval
pip install -r requirements.txt
python run_eval.py   # outputs MRR, NDCG@10, Hit Rate
```
