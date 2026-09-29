# CineMatch Ranker

Stage-2 re-ranking service. It takes the 50 candidates from pgvector retrieval and re-scores them with movie features and user preferences. Only the Go backend calls it. The Cloud Run URL is public, but the service holds no data and only scores the candidates it is sent.

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
    {
      "movie_id": "uuid",
      "score": 0.847,
      "rank": 1,
      "factors": [
        {"feature": "similarity", "contribution": 0.2135},
        {"feature": "vote_average", "contribution": 0.0412}
      ]
    }
  ],
  "model_version": "lambdamart-v1"
}
```

`factors` explains each pick. They are LightGBM's SHAP values (`pred_contrib=True`) for the title-level features that raised the score, largest first, at most three. User-level features are the same for every candidate in a request, so they are left out. `feature-linear-v1` returns no factors.

Every `user_features` field is optional. The Go backend sends `user_like_ratio` and `user_interaction_count`, the two lambdamart-v1 uses; `preferred_genres` and `min_vote_preference` only affect feature-linear-v1.

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

**Vote-floor penalty:** If a movie's rating falls below the user's `min_vote_preference`, the score is halved. A title strong on every other signal can still rank.

**Genre overlap with no preferences:** Returns 0.5 (neutral) so cold-start users are not penalized.

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

## Docker

```bash
docker build -t cinematch-ranker .
docker run -p 8000:8000 cinematch-ranker
```

Base image: `python:3.13-slim`. Cloud Run (or Render) injects the `PORT` env var at runtime, and the trained model is bundled at `model/lambdamart-v1.txt`.
