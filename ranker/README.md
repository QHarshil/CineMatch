# CineMatch Ranker

Stage-2 re-ranking microservice. Takes the 50 candidates that pgvector retrieval produces and re-scores them using movie features and user preferences. The Go backend calls this internally; it's not exposed to browsers.

## Running locally

```bash
cd ranker
pip install -r requirements.txt
uvicorn main:app --reload --port 8000
```

Run tests:

```bash
pytest tests/ -v
```

Environment variables:

| Variable | Required | Default |
|----------|----------|---------|
| `HOST` | no | `127.0.0.1` |
| `PORT` | no | `8000` |
| `APP_ENV` | no | `development` |
| `LAMBDAMART_MODEL_PATH` | no | bundled `model/lambdamart-v1.txt`, falls back to `../eval/models/` |

In production, `APP_ENV=production` disables the `/docs` endpoint.

## API

### POST /rank

Re-rank Stage-1 candidates.

Request:
```json
{
  "candidates": [
    {
      "movie_id": "uuid",
      "title": "Inception",
      "genres": ["Action", "Science Fiction"],
      "release_year": 2010,
      "vote_average": 8.4,
      "popularity": 99.9,
      "runtime": 148,
      "similarity": 0.92
    }
  ],
  "user_features": {
    "preferred_genres": ["Science Fiction", "Thriller"],
    "min_vote_preference": 7.0,
    "user_like_ratio": 0.62,
    "user_interaction_count": 40
  },
  "top_n": 20,
  "model": "lambdamart-v1"
}
```

Response:
```json
{
  "ranked": [
    {"movie_id": "uuid", "score": 0.847, "rank": 1}
  ],
  "model_version": "lambdamart-v1"
}
```

The `model` field selects which ranker to use: `lambdamart-v1` (the production default) or `feature-linear-v1` (the transparent fallback). Both are described under Scoring models below.

### GET /health

Returns `{"status": "ok", "service": "cinematch-ranker"}`.

## Scoring models

### feature-linear-v1

A weighted linear combination of four signals:

```
score = 0.50 * similarity
      + 0.25 * (vote_average / 10)
      + 0.15 * min(log1p(popularity) / 8.01, 1.0)
      + 0.10 * genre_overlap
```

| Signal | Weight | Source |
|--------|--------|--------|
| Similarity | 0.50 | Cosine similarity from pgvector kNN |
| Quality | 0.25 | TMDB vote_average normalized to [0, 1] |
| Popularity | 0.15 | Log-scaled, capped at log1p(3000) to prevent blockbusters from dominating |
| Genre overlap | 0.10 | Fraction of candidate genres matching user preferences |

I chose these weights by intuition and manual testing. Similarity gets the most weight because if the vector search thinks a movie matches, it probably does. Quality and popularity prevent obscure low-rated movies from ranking high just because their embedding happens to be close.

**Vote-floor penalty:** If a movie's rating falls below the user's `min_vote_preference`, the score is halved. This is a soft penalty, not a hard cutoff, because a movie that's great in every other dimension shouldn't be completely buried by a mediocre rating.

**Genre overlap with no preferences:** Returns 0.5 (neutral) when the user hasn't expressed genre preferences yet. This avoids penalizing cold-start users.

### lambdamart-v1

The production re-ranker: a LightGBM model trained with the `lambdarank` objective. Every feature is computed identically in training (`eval/build_training_data.py`) and at serve time (`lambdamart_ranker.py`), so there is no train/serve skew:

| Feature | Description |
|---------|-------------|
| similarity | Retrieval score: pgvector cosine similarity at serve time (the synthetic genre affinity during training) |
| vote_average | TMDB rating [0, 10] |
| log_popularity | log1p of TMDB popularity |
| decade | Release decade as ordinal (1970=0, 1980=1, ...) |
| is_recent | 1 if released >= 2021 |
| user_like_ratio | Fraction of the user's interactions that are "like" (sent by the Go backend) |
| user_interaction_count | Total interactions for this user (sent by the Go backend) |

The feature vector in `lambdamart_ranker.py` must exactly match `FEATURE_COLUMNS` in `eval/build_training_data.py`. If you add or reorder features in training, update the ranker too. On synthetic eval this model leads on NDCG@10 (0.814, +14% over a popularity baseline).

Training details: 200 boost rounds, 31 leaves, learning rate 0.05, lambdarank truncation at 10. See `eval/train_lambdamart.py` for the full config.

## Upgrade path

The current setup makes it straightforward to swap or A/B test models:

1. Train a new model in `eval/` (change features, hyperparameters, or training data).
2. Export it to `eval/models/your-model-v2.txt`.
3. Add a new scoring function in `ranker/` that loads and applies it.
4. Add the model name to the routing logic in `main.py`.
5. Update the Go backend to request the new model name in the `model` field.

The `model` field in the request body means the Go backend controls which model scores each request. This makes it possible to run A/B experiments by routing a percentage of traffic to the new model and comparing conversion metrics.

## Docker

```bash
docker build -t cinematch-ranker .
docker run -p 8000:8000 cinematch-ranker
```

Base image: `python:3.12-slim`. Cloud Run (or Render) injects the `PORT` env var at runtime, and the trained model is bundled at `model/lambdamart-v1.txt`.
