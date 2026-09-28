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
| `PORT` | no | injected by Cloud Run/Render; falls back to `APP_PORT` then `8080` |
| `APP_PORT` | no | `8080` |
| `ALLOWED_ORIGINS` | no | `http://localhost:3000` |
| `RATE_LIMIT_RPM` | no | `60` |
| `OMDB_API_KEY` | no | - (IMDb/Rotten Tomatoes ratings are hidden if unset) |
| `OPENAI_API_KEY` | no | - (query embeddings for `/discover`; search runs keyword-only if unset) |
| `EMBED_DAILY_LIMIT` | no | `5000` upstream embedding calls per instance per UTC day |
| `LLM_BASE_URL` | no | - (OpenAI-compatible chat API, e.g. `http://localhost:11434/v1`; the assistant answers from search alone if unset) |
| `LLM_MODEL` | no | - (e.g. `qwen3:8b` on Ollama) |
| `LLM_API_KEY` | no | - (empty for local Ollama) |
| `LLM_REASONING_EFFORT` | no | - (`none` turns off thinking on reasoning models; omit for providers that reject the field) |
| `ASSISTANT_USER_DAILY_RUNS` | no | `25` assistant requests per user per UTC day |
| `ASSISTANT_GUEST_DAILY_RUNS` | no | `8` for guest (anonymous) sessions, which anyone can create |
| `ASSISTANT_DAILY_RUNS` | no | `1000` model-backed runs per UTC day across all users |
| `ASSISTANT_DAILY_TOKENS` | no | `2000000` model tokens per UTC day across all users |

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
    "movie_count": 1510,
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
  "media_type": "movie",
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

**GET /movies/{id}/ratings**

Aggregate IMDb and Rotten Tomatoes scores from OMDb, looked up by title, year, and type, then cached in memory (24h TTL). Either score may be `null` when OMDb has no value for it; the body is empty (both `null`) when `OMDB_API_KEY` is unset or OMDb is unreachable, so the detail page degrades gracefully.

```json
{ "imdb_rating": 8.4, "rt_rating": 87 }
```

**GET /search?q=inception&limit=20**

Title search using Postgres ILIKE backed by a trigram GIN index. `q` is 1-200 characters, `limit` capped at 50. Falls back to filtering cached popular movies when the database is down. Rate limited to 30 req/min per IP.

**GET /discover?q=slow-burn+sci-fi&type=tv&limit=20**

Natural-language search. The query is embedded with `text-embedding-3-small` (the model that embedded the catalog) and the `search_titles_hybrid` RPC fuses three rankers with reciprocal rank fusion: pgvector cosine similarity, Postgres full-text rank over title and overview, and trigram title similarity for typos. Rate limited to 30 req/min per IP.

| Param | Validation |
|-------|------------|
| `q` | required, 1-200 characters, HTML stripped |
| `type` | `movie` or `tv` |
| `genre` | comma-separated, at most 5 |
| `year_min`, `year_max` | 1900-2100, min not above max |
| `rating_min` | 0-10 (TMDB vote average) |
| `runtime_max` | 1-600 minutes |
| `lang` | two-letter ISO 639-1 code |
| `limit` | 1-50, default 20 |

```json
{
  "retrieval": "hybrid",
  "results": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Inception",
      "media_type": "movie",
      "release_year": 2010,
      "original_language": "en",
      "similarity": 0.515,
      "semantic_rank": 1,
      "keyword_rank": null,
      "title_rank": null,
      "score": 0.0164
    }
  ]
}
```

A `null` rank means that ranker did not return the title. `retrieval` is `hybrid` when the query was embedded, `keyword` when no embedding was available (no key, daily cap reached, or OpenAI down), and `cached` when Supabase was unreachable and results come from the popular cache.

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
  "model_version": "lambdamart-v1",
  "explanations": {
    "550e8400-e29b-41d4-a716-446655440000": {
      "similarity": 0.71,
      "because_you_liked": { "id": "...", "title": "Arrival", "similarity": 0.83 },
      "factors": [{ "feature": "similarity", "contribution": 0.2135 }]
    }
  }
}
```

Titles the user already liked, watched, disliked, or skipped are excluded; the pipeline over-fetches from pgvector to keep 50 candidates. Each pick carries an explanation:
- `similarity` is the cosine similarity to the taste vector.
- `because_you_liked` is the liked title closest to the pick, from the `nearest_liked_titles` RPC.
- `factors` are the ranker's SHAP contributions.

Popular feeds have no explanations.

**POST /interactions**

Toggles a user signal. Rate limited to 20 req/min per user, capped at 500 total interactions per user. Sending a signal that already exists removes it; `like` and `dislike` are mutually exclusive (setting one clears the other). After any change the user's taste embedding is rebuilt so `/recommend` reflects it.

Request:
```json
{
  "movie_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "like"
}
```

Valid types: `like`, `dislike`, `watch`, `skip`. Unknown JSON fields are rejected. All string inputs are sanitized to strip HTML tags.

Returns `200 OK` with the resulting action, or `429 Too Many Requests` when the cap is hit:
```json
{ "action": "added", "type": "like" }
```

**GET /interactions?movie_id={uuid}**

Returns the user's active interactions and rating for one movie.
```json
{ "interactions": ["like", "watch"], "rating": 8 }
```

**PUT /ratings**

Upserts a 1-10 star rating for a movie (send `score: 0` to clear it). Rate limited to 20 req/min per user.
```json
{ "movie_id": "550e8400-e29b-41d4-a716-446655440000", "score": 8 }
```

**POST /assistant**

The grounded assistant. Rate limited to 6 req/min per user, plus the daily caps above. Body:

```json
{
  "messages": [
    { "role": "user", "content": "Something like Parasite, but a series" }
  ]
}
```

Up to 12 turns (`user` or `assistant`), the last one from the user; each user turn is at most 800 characters. Older turns are dropped once the conversation passes 3,000 characters. The response is a `text/event-stream`:

| Event | Data |
|-------|------|
| `start` | `{ "model", "prompt_version" }` |
| `tool_call` | `{ "id", "tool", "label", "args" }`, sent before the tool runs |
| `tool_result` | `{ "id", "tool", "count", "latency_ms", "retrieval", "titles", "error" }` |
| `picks` | `{ "message", "picks": [{ "movie", "reason", "similarity", "source" }], "dropped" }` |
| `message` | `{ "text" }`: a clarifying question or a decline |
| `error` | `{ "code", "message" }` |
| `done` | `{ "run_id", "status", "model", "usage": { "input_tokens", "output_tokens" }, "latency_ms", "remaining_today" }` |

`status` is `picks`, `answered`, `fallback` (search results served without the model's final answer), or `error`. Returns 429 with `{ "error", "resets_at" }` once the user's daily limit is reached, and 503 when usage cannot be checked (the quota fails closed).

**GET /assistant/usage**

```json
{ "used": 3, "limit": 25, "remaining": 22, "resets_at": "2026-09-29T00:00:00Z", "model_available": true }
```

`model_available` is false when no model is configured or the global budget is spent; the assistant then answers from search.

## Architecture decisions

**Why Chi.** Chi's middleware composes as `func(http.Handler) http.Handler`, which is the stdlib pattern. No framework lock-in, no magic. Route groups (`r.Group`) make it clean to apply auth middleware to authenticated routes without touching public ones.

**Middleware stack.** The 9-layer stack runs in this order, and the order matters:

1. `RequestID` - assigns a unique ID for log correlation
2. `RealIP` - extracts the real client IP from proxy headers (must run before rate limiting)
3. `StructuredLogger` - JSON log per request: method, path, status, latency_ms, bytes, request_id, remote_addr
4. `Recoverer` - catches panics so one bad request doesn't crash the server
5. `CORSHandler` - reads `ALLOWED_ORIGINS`, allows GET/POST/PUT/DELETE/OPTIONS
6. `RateLimiter` - global 60 req/min per IP
7. `SecurityHeaders` - X-Content-Type-Options nosniff, X-Frame-Options DENY, Cache-Control no-store
8. `RequireJSONContentType` - rejects POST/PUT/PATCH without `application/json` (415)
9. `MaxBodySize` - rejects request bodies over 10KB (413)

RequestID and RealIP come first because the logger and rate limiter need accurate data.

**Two-stage pipeline wiring.** The recommend handler checks `GetUserEmbedding` first. No embedding means cold start, so it returns popular movies immediately and skips the whole pipeline. If an embedding exists, it calls `MatchMovies` (pgvector RPC, 50 candidates), then POSTs those to the Python ranker. If the ranker is down, candidates come back in similarity order. The frontend doesn't need to know about the failure.

**Graceful degradation.** A `PopularMoviesCache` holds the top 50 movies in memory, refreshed hourly. When Supabase is unreachable, `/movies`, `/search`, `/discover`, and `/recommend` all fall back to this cache instead of returning 500s. Search does a basic title substring match against cached movies. This keeps the site functional during database maintenance or outages.

**Hybrid retrieval.** Embeddings capture mood and plot ("a heist that goes wrong") but miss exact titles and typos; full-text and trigram matching catch those. Reciprocal rank fusion combines them by rank, so the three scores never need calibrating against each other. Filters are applied inside each ranker, and pgvector 0.8 iterative index scans keep the HNSW search walking until enough rows pass them. Query embeddings go through an LRU cache and a per-day call cap (`embed.Budgeted`), so repeated queries are free and spend is bounded even under a traffic spike.

**Grounded assistant.** The agent (`assistant/`) runs a tool-calling loop against any OpenAI-compatible chat API (`llm/`), so Ollama, Groq, Gemini, and OpenAI differ only by `LLM_BASE_URL`, `LLM_MODEL`, and `LLM_API_KEY`. Its tools are read-only: `search_catalog` (hybrid search with filters), `find_similar` (filtered vector search from one title), `get_taste_profile`, `get_recommendations` (the two-stage pipeline), and `present_picks`, which ends the run. Every title a tool returns gets a short ref (`t3`), and `present_picks` only accepts refs the agent has seen, so a model cannot recommend a title outside the catalog. Rejected refs are counted in the audit log. Short refs matter because small models copy them reliably where they garble UUIDs.

The agent also guards against common small-model failures:
- It maps genre words across TMDB's film and series vocabularies. Series have no Thriller genre, for example.
- It accepts quoted numbers.
- When a filter hides a title the person named, it returns that title separately as a seed.
- If the model answers in prose after using tools, it asks once for `present_picks`, then falls back to the grounded titles.

If the model is unconfigured, rate limited, down, or over the daily budget, the endpoint still answers with hybrid search results and says so.

**Output guard.** In the eval, a "developer mode" prompt got qwen3:8b to repeat part of its system prompt, so wording the instructions more firmly was not enough. Before any reply or picks message is sent, it is checked for tool names, section headings, and any eight-word run copied from the instructions. A match is replaced with a plain decline and recorded as `output_blocked` in the audit log.

**Assistant audit and quotas.** Each run writes an `assistant_runs` row with the model, prompt version, tool calls, pick IDs, grounding drops, whether the output guard fired, token counts, and latency. The prompt is stored only as a SHA-256 hash and a length. Tool arguments are stored with emails and phone numbers masked. The same table backs the per-user daily limit and the global run and token caps, checked in one RPC before any model call. Guest sessions (Supabase anonymous sign-in, flagged by the token's `is_anonymous` claim) get a smaller limit. Rows are readable by their owner under RLS and writable only by the service role.

**Interaction caps.** Each user can record at most 500 interactions total. Enforced in the Go handler (fast fail before the DB round-trip) and via a Supabase RLS INSERT policy (database-level safety net). This prevents a single account from flooding the interactions table on the free tier.

## Docker

```bash
docker build -t cinematch-backend .
docker run -p 8080:8080 --env-file ../.env cinematch-backend
```

Multi-stage build: `golang:1.22-alpine` for compilation, `distroless/static-debian12` for the runtime. The final binary is fully static (CGO disabled), so the runtime image has no shell or package manager.
