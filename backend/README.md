# CineMatch Backend

Go API server powering search and recommendations for CineMatch. Built with Chi router, backed by Supabase Postgres + pgvector, and wired to the Python ranking microservice.

## Architecture role

```
Frontend (Next.js)
       |
       | REST calls
       v
Backend (this service — Go + Chi)
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

## Endpoints

| Method | Path      | Description                                   |
|--------|-----------|-----------------------------------------------|
| GET    | /health   | Liveness check + Supabase reachability        |

Additional endpoints added in Task 3: `/movies`, `/movies/:id`, `/search`, `/recommend/:userId`, `/interactions`.

## Environment variables

| Variable               | Required | Description                                              |
|------------------------|----------|----------------------------------------------------------|
| `SUPABASE_URL`         | yes      | Supabase project URL (https://xyz.supabase.co)           |
| `SUPABASE_SECRET_KEY`  | yes      | Service-role key. Never expose to frontend or git.       |
| `APP_PORT`             | no       | HTTP listen port (default: 8080)                         |
| `ALLOWED_ORIGINS`      | yes      | Comma-separated CORS origins                             |
| `RATE_LIMIT_RPM`       | no       | Requests per minute per IP (default: 60)                 |

## Deploying to Railway

1. Push this directory to a Railway service.
2. Set the environment variables above in the Railway dashboard.
3. Railway auto-detects the Dockerfile and builds/deploys on push.
