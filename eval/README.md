# CineMatch Eval Pipeline

Offline evaluation pipeline for comparing Stage-2 ranker models. Generates synthetic user interaction data, trains a LambdaMART model, and benchmarks it against the feature-linear-v1 baseline.

## Pipeline Steps

```
1. generate_synthetic_users.py   → eval/data/synthetic_interactions.parquet
2. build_training_data.py        → eval/data/train.parquet, test.parquet
3. train_lambdamart.py           → eval/models/lambdamart-v1.txt
4. eval_rankers.py               → eval/results/eval_report.json
```

## Synthetic Users

200 users across 8 taste profiles (action_fan, arthouse, comedy_lover, horror_buff, scifi_nerd, drama_enthusiast, thriller_junkie, generalist). Each user gets 20-80 interactions against the real 494-movie catalog from Supabase, weighted by genre affinity with Gaussian noise.

Interaction types: `like` (relevance=3), `watch` (2), `skip` (1), `dislike` (0).

## Features (10 total)

| Feature | Description |
|---------|-------------|
| `affinity_score` | Genre affinity between movie and user profile |
| `vote_average` | TMDB average rating |
| `log_popularity` | log1p of TMDB popularity |
| `runtime_hours` | Runtime in hours |
| `decade` | Release decade as ordinal (1970=0) |
| `genre_count` | Number of genres on the movie |
| `is_recent` | 1 if released >= 2021 |
| `user_avg_affinity` | Mean affinity across user's interactions |
| `user_like_ratio` | Fraction of user's interactions that are "like" |
| `user_interaction_count` | Total interactions for this user |

## Models

- **feature-linear-v1**: Explicit weighted formula (50% similarity + 25% quality + 15% log-popularity + 10% genre overlap)
- **lambdamart-v1**: LightGBM LambdaMART trained on synthetic interactions (200 boost rounds, 31 leaves)

## Metrics

| Metric | Description |
|--------|-------------|
| NDCG@10 | Normalized Discounted Cumulative Gain at rank 10 |
| MRR | Mean Reciprocal Rank of first relevant item |
| Hit Rate@10 | Fraction of users with at least one relevant item in top 10 |

## Running

```bash
cd eval
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt pyarrow pytest

# Generate data (requires SUPABASE_URL + SUPABASE_SECRET_KEY in ../.env)
python generate_synthetic_users.py
python build_training_data.py

# Train
python train_lambdamart.py

# Evaluate
python eval_rankers.py

# Tests
python -m pytest tests/ -v
```

## Requirements

- Python 3.12+
- lightgbm, pandas, scikit-learn, numpy, pyarrow
- Supabase credentials (for initial data fetch only)
