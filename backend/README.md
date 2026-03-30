# CineMatch Backend

Go API server that coordinates search, recommendations, and user interactions between the Next.js frontend, Supabase, and the Python ranker.

## Running locally

```bash
cd backend
go run .
# Listening on :8080
```

Required env vars (set in `../.env` or export directly):

| Variable | Required | Default |
|----------|----------|---------|
| `SUPABASE_URL` | yes | - |
| `SUPABASE_SECRET_KEY` | yes | - |
| `JWT_SECRET` | yes | - |
| `RANKER_URL` | no | `http://localhost:8000` |
| `APP_PORT` | no | `8080` |
| `ALLOWED_ORIGINS` | no | `http://localhost:3000` |
| `RATE_LIMIT_RPM` | no | `60` |

Run tests:

```bash
go test ./...
```

## API endpoints

### Public (no auth)

**GET /health**

Returns service status and database row counts for free-tier monitoring.

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime_seconds": 3421.5,
  "database": "ok",
  "stats": {
    "movie_count": 494,
    "user_count": 12,
    "interaction_count": 847
  }
}
```

`stats` is omitted when the database is unreachable.

**GET /movies?limit=20&offset=0**

Paginated movie list ordered by popularity. `limit` capped at 100. Falls back to an in-memory cache if Supabase is unreachable.

**GET /movies/{id}**

Single movie by UUID. Returns 404 if not found.

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "tmdb_id": 27205,
  "title": "Inception",
  "overview": "...",
  "genres": ["Action", "Science Fiction", "Adventure"],
  "release_year": 2010,
  "poster_path": "/9gk7adHYeDvHkCSEhniW0WCbiRl.jpg",
  "backdrop_path": "/s3TBrRGB1iav7gFOCNx3H31MoES.jpg",
  "vote_average": 8.364,
  "popularity": 99.9,
  "runtime": 148
}
```

**GET /search?q=inception&limit=20**

Title search using Postgres ILIKE backed by a trigram GIN index. `q` is 1-200 characters, `limit` capped at 50. Falls back to filtering cached popular movies when the database is down. Rate limited to 30 req/min per IP.

### Authenticated (require `Authorization: Bearer <supabase-jwt>`)

**GET /recommend**

Two-stage recommendation pipeline. Rate limited to 10 req/min per user.

The response includes a `source` field so the frontend knows what it got:

| source | meaning |
|--------|---------|
| `personalized` | Full pipeline ran: pgvector retrieval then ranker re-scoring |
| `similarity_fallback` | Ranker was unreachable; results in pgvector cosine order |
| `popular` | User has no interaction history yet (cold start) |

```json
{
  "movies": [ ... ],
  "source": "personalized",
  "model_version": "feature-linear-v1"
}
```

**POST /interactions**

Records a user signal. Rate limited to 20 req/min per user. Capped at 500 total interactions per user and 5 per movie.

Request:
```json
{
  "movie_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "like"
}
```

Valid types: `like`, `dislike`, `watch`, `skip`. Unknown JSON fields are rejected. All string inputs are sanitized to strip HTML tags.

Returns `201 Created` on success, `429 Too Many Requests` when caps are hit.

## Architecture decisions

**Why Chi.** Chi's middleware composes as `func(http.Handler) http.Handler`, which is the stdlib pattern. No framework lock-in, no magic. Route groups (`r.Group`) make it clean to apply auth middleware to authenticated routes without touching public ones.

**Middleware stack.** The 9-layer stack runs in this order, and the order matters:

1. `RequestID` - assigns a unique ID for log correlation
2. `RealIP` - extracts the real client IP from proxy headers (must run before rate limiting)
3. `StructuredLogger` - JSON log per request: method, path, status, latency_ms, bytes, request_id, remote_addr
4. `Recoverer` - catches panics so one bad request doesn't crash the server
5. `CORSHandler` - reads `ALLOWED_ORIGINS`, allows GET/POST/OPTIONS only
6. `RateLimiter` - global 60 req/min per IP
7. `SecurityHeaders` - X-Content-Type-Options nosniff, X-Frame-Options DENY, Cache-Control no-store
8. `RequireJSONContentType` - rejects POST/PUT/PATCH without `application/json` (415)
9. `MaxBodySize` - rejects request bodies over 10KB (413)

RequestID and RealIP come first because the logger and rate limiter need accurate data.

**Two-stage pipeline wiring.** The recommend handler checks `GetUserEmbedding` first. No embedding means cold start, so it returns popular movies immediately and skips the whole pipeline. If an embedding exists, it calls `MatchMovies` (pgvector RPC, 50 candidates), then POSTs those to the Python ranker. If the ranker is down, candidates come back in similarity order. The frontend doesn't need to know about the failure.

**Graceful degradation.** A `PopularMoviesCache` holds the top 50 movies in memory, refreshed hourly. When Supabase is unreachable, `/movies`, `/search`, and `/recommend` all fall back to this cache instead of returning 500s. Search does a basic title substring match against cached movies. This keeps the site functional during database maintenance or outages.

**Interaction caps.** Each user can record at most 500 interactions total and 5 per movie. Enforced in the Go handler (fast fail before the DB round-trip) and via a Supabase RLS INSERT policy (database-level safety net). This prevents a single account from flooding the interactions table on the free tier.

## Docker

```bash
docker build -t cinematch-backend .
docker run -p 8080:8080 --env-file ../.env cinematch-backend
```

Multi-stage build: `golang:1.22-alpine` for compilation, `distroless/static-debian12` for the runtime. The final binary is fully static (CGO disabled), so the runtime image has no shell or package manager.
