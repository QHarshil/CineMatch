# CineMatch Backend

Go API server powering search and recommendations for CineMatch. Built with Chi router, backed by Supabase Postgres + pgvector, and wired to the Python ranking microservice.

## Architecture role

```
Frontend (Next.js)
       |
       | REST calls
       v
Backend (this service -- Go + Chi)
       |                    |
       | Supabase REST/RPC  | HTTP POST /rank
       v                    v
  Supabase Postgres     Python Ranker
  (pgvector + RLS)      (FastAPI)
```

The backend never returns raw embeddings to the frontend. It handles all OpenAI and Supabase service-key calls server-side.

## Running locally

```bash
cp ../.env.example ../.env   # fill in secrets
go mod download
go run .
```

Server starts on `APP_PORT` (default `8080`).

## Running tests

```bash
go test ./...
```

35 tests covering all handlers, middleware, and the ranker HTTP client.

## Endpoints

### Public

| Method | Path          | Description                                      |
|--------|---------------|--------------------------------------------------|
| GET    | /health       | Liveness check + Supabase reachability           |
| GET    | /movies       | Paginated movie list (limit 1-100, offset)       |
| GET    | /movies/{id}  | Single movie by UUID                             |
| GET    | /search?q=    | Title search (ILIKE, limit 1-50)                 |

### Authenticated (requires Supabase JWT in `Authorization: Bearer <token>`)

| Method | Path           | Description                                      |
|--------|----------------|--------------------------------------------------|
| GET    | /recommend     | Two-stage recommendations for the logged-in user |
| POST   | /interactions  | Record a user signal (like/dislike/watch/skip)   |

### GET /recommend

Two-stage recommendation pipeline:

1. **Stage 1 (pgvector):** fetches user embedding, calls `match_movies` RPC for top-50 candidates by cosine similarity.
2. **Stage 2 (ranker):** POSTs candidates to the Python ranker at `RANKER_URL/rank`, which re-scores and returns top-20.

**Fallback behavior:**
- Cold-start users (no embedding): returns popular movies sorted by popularity, `source: "popular"`.
- Ranker unreachable: returns candidates in cosine-similarity order, `source: "similarity_fallback"`.
- Ranker succeeds: returns re-ranked movies, `source: "personalized"`, `model_version: "feature-linear-v1"`.

Response shape:
```json
{
  "movies": [{ "id": "...", "title": "...", ... }],
  "source": "personalized",
  "model_version": "feature-linear-v1"
}
```

### POST /interactions

```json
{ "movie_id": "uuid", "type": "like" }
```
Type must be one of: `like`, `dislike`, `watch`, `skip`.

## Environment variables

| Variable               | Required | Description                                              |
|------------------------|----------|----------------------------------------------------------|
| `SUPABASE_URL`         | yes      | Supabase project URL (https://xyz.supabase.co)           |
| `SUPABASE_SECRET_KEY`  | yes      | Service-role key. Never expose to frontend or git.       |
| `JWT_SECRET`           | yes      | Supabase JWT secret for HS256 verification               |
| `RANKER_URL`           | no       | Python ranker base URL (default: http://localhost:8000)   |
| `APP_PORT`             | no       | HTTP listen port (default: 8080)                         |
| `ALLOWED_ORIGINS`      | yes      | Comma-separated CORS origins                             |
| `RATE_LIMIT_RPM`       | no       | Requests per minute per IP (default: 60)                 |

## Deploying to Railway

1. Push this directory to a Railway service.
2. Set the environment variables above in the Railway dashboard.
3. Set `RANKER_URL` to the internal Railway URL of the Python ranker service.
4. Railway auto-detects the Dockerfile and builds/deploys on push.
