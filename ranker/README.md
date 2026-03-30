# CineMatch Ranker

Stage-2 re-ranking microservice for the CineMatch recommendation pipeline.

## Role in the pipeline

```
Go backend
  └── Stage 1: match_movies RPC → top-50 candidates (pgvector cosine kNN)
        └── Stage 2: POST /rank → top-20 re-ranked results  ← this service
```

The Go backend calls `POST /rank` after retrieving Stage-1 candidates from Supabase.
The ranker re-scores each candidate using a composite feature score and returns the top-N results.

## Scoring model (`feature-linear-v1`)

A weighted linear combination of four signals:

| Signal | Weight | Notes |
|---|---|---|
| `similarity` | 0.50 | Cosine similarity from pgvector kNN |
| `vote_average` | 0.25 | Normalised to [0, 1] (÷10) |
| `popularity` | 0.15 | Log-scaled against a 3000-ceiling to prevent blockbusters dominating |
| `genre_overlap` | 0.10 | Fraction of candidate genres matching user's preferred genres |

An optional vote-floor penalty (×0.5) applies when `vote_average` falls below
the user's `min_vote_preference` threshold.

**Upgrade path:** once the offline eval pipeline (Task 11) produces sufficient
interaction data, swap `ranker.py`'s `rank()` for a `lightgbm.LGBMRanker`
fitted on `(query, candidate, label)` triples. The `POST /rank` interface stays the same.

## API

### `POST /rank`

**Request**
```json
{
  "candidates": [
    {
      "movie_id": "uuid",
      "title": "Inception",
      "genres": ["Action", "Science Fiction"],
      "release_year": 2010,
      "vote_average": 8.8,
      "popularity": 850.0,
      "runtime": 148,
      "similarity": 0.92
    }
  ],
  "user_features": {
    "preferred_genres": ["Action", "Science Fiction"],
    "min_vote_preference": 7.0
  },
  "top_n": 20
}
```

**Response**
```json
{
  "ranked": [
    { "movie_id": "uuid", "score": 0.834512, "rank": 1 }
  ],
  "model_version": "feature-linear-v1"
}
```

Validation: `candidates` must be non-empty; `top_n` is in [1, 50] (default 20).

### `GET /health`

Returns `{"status": "ok", "service": "cinematch-ranker"}` with HTTP 200.

## Running locally

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --reload --port 8000
```

Service available at `http://localhost:8000`. Interactive docs at `http://localhost:8000/docs`.

## Running tests

```bash
source .venv/bin/activate
pytest tests/ -v
```

23 tests covering scoring helpers, rank ordering, edge cases, and HTTP endpoints.

## Deployment (Railway)

The `Dockerfile` builds a single-stage Python image. Railway injects `PORT` at runtime.

Environment variables required: none (this service is stateless and has no secrets).

The Go backend locates this service via `RANKER_URL` env var (set in Railway).
