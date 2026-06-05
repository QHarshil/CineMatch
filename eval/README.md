# CineMatch Eval Pipeline

Offline evaluation for comparing ranker models. Generates synthetic user data, trains a LambdaMART model, and benchmarks it against the feature-linear baseline.

I built this because I needed a way to measure whether changes to the ranking model actually improve recommendations before deploying them. Online A/B testing requires real traffic, so this synthetic pipeline gives a reasonable signal during development.

## Running the full pipeline

```bash
cd eval
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

python generate_synthetic_users.py     # ~10 seconds
python build_training_data.py          # ~5 seconds
python train_lambdamart.py             # ~15 seconds
python eval_rankers.py                 # ~10 seconds
```

Measure re-ranking latency (needs a ranker running locally on port 8000):

```bash
python benchmark_latency.py
```

Run tests:

```bash
python -m pytest tests/ -v             # 20 tests
```

Output files:
- `data/synthetic_interactions.parquet` -- 8,871 interactions across 200 users
- `data/train.parquet`, `data/test.parquet` -- feature-engineered training data
- `models/lambdamart-v1.txt` -- trained LightGBM model
- `results/eval_report.json` -- metric comparison

## Metrics

Three metrics, each measuring something different:

**NDCG@10** (Normalized Discounted Cumulative Gain) -- the primary metric. Measures ranking quality by checking whether relevant movies appear near the top. A movie at position 1 contributes more than one at position 10 because of the logarithmic discount. NDCG of 1.0 means perfect ranking; 0.0 means nothing relevant in the top 10.

Formula: `DCG = sum(gain_i / log2(i + 2))`, normalized by the ideal DCG (if you sorted by relevance first).

**MRR** (Mean Reciprocal Rank) -- how quickly the first relevant result appears. If the first "like" or "watch" movie is at position 3, the reciprocal rank is 1/3. Averaged across all users. High MRR means users don't have to scroll far to find something good.

**Hit Rate@10** -- the simplest metric. What fraction of users see at least one relevant movie in their top 10? A sanity check: if this is low, the pipeline is fundamentally broken.

Relevance is defined as interactions of type "like" or "watch" (relevance label >= 2).

## Results

Most recent eval (held-out: 40 users, 1,695 interactions), produced by `eval_rankers.py`:

| Model | NDCG@10 | MRR | Hit Rate@10 |
|-------|---------|-----|-------------|
| Popularity baseline | 0.716 | 0.875 | 1.00 |
| Vector retrieval only | 0.798 | 0.938 | 1.00 |
| Two-stage (feature-linear-v1) | 0.795 | 0.950 | 1.00 |
| Two-stage (lambdamart-v1) | 0.814 | 1.000 | 1.00 |

LambdaMART leads on NDCG@10: +14% over the popularity baseline, and ahead of both retrieval-only and the linear re-ranker. The synthetic users have non-linear preferences (a favourite release era, a vote-average sweet spot, and recency that only applies inside loved genres) that a fixed-weight linear formula structurally cannot represent.

Note the linear re-ranker (0.795) does not beat retrieval-only (0.798): its monotonic quality weight misranks users whose taste peaks at mid-range ratings. That is exactly the non-monotonic relationship the learned model captures, and it is the argument for a learned ranker over hand-tuned weights.

Re-ranking latency (50 candidates to top 20, `benchmark_latency.py`): p50 0.8 ms, p95 0.9 ms, p99 1.0 ms.

## Synthetic data generation

`generate_synthetic_users.py` creates 200 users across 8 taste profiles:

| Profile | Weight | Loved genres |
|---------|--------|-------------|
| action_fan | 15% | Action, Adventure, Science Fiction |
| arthouse | 12% | Drama, History, Documentary |
| comedy_lover | 14% | Comedy, Animation, Family |
| horror_buff | 10% | Horror, Thriller, Mystery |
| scifi_nerd | 12% | Science Fiction, Fantasy, Adventure |
| drama_enthusiast | 13% | Drama, Romance, Crime |
| thriller_junkie | 10% | Thriller, Crime, Mystery |
| generalist | 14% | No strong preference |

Each user generates 20-80 interactions against the real 494-movie catalog. Which movies a user engages with, and whether they like / watch / skip / dislike, is driven by a "true utility" that combines a linear genre signal with three deliberately non-linear effects: a favourite release decade (a peaked bump), a vote-average sweet spot rather than "higher is always better," and a recency bias that only applies inside loved genres. The genre signal alone is stored as the retrieval similarity, so a ranker limited to it leaves the non-linear structure on the table. Gaussian noise keeps outcomes non-deterministic.

## Feature engineering

`build_training_data.py` computes features that are engineered identically here
and in the live ranker (`ranker/lambdamart_ranker.py`), so the model sees the
same inputs in training and production (no train/serve skew):

| Feature | Description |
|---------|-------------|
| similarity | Retrieval score: synthetic genre affinity in training, pgvector cosine similarity in production |
| vote_average | TMDB average rating [0, 10] |
| log_popularity | log1p of TMDB popularity score |
| decade | Release decade as ordinal (1970s=0, 1980s=1, ...) |
| is_recent | Binary: 1 if released >= 2021 |
| user_like_ratio | Fraction of the user's interactions that are "like" |
| user_interaction_count | Total interactions for this user |

Train/test split is 80/20 by user, not by interaction. This prevents leakage: a user's training interactions can't inform predictions about that same user's test interactions.

Relevance labels: like=3, watch=2, skip=1, dislike=0.

## LambdaMART training

`train_lambdamart.py` trains a LightGBM model with these key settings:

- Objective: `lambdarank` (optimizes NDCG directly)
- 200 boost rounds, 31 leaves per tree, max depth 6
- Learning rate: 0.05
- Feature/bagging fraction: 0.8 (regularization against overfitting)
- Truncation level: 10 (only the top 10 positions affect the loss)

The model file is saved to `models/lambdamart-v1.txt` in LightGBM's native format, which the ranker service loads at startup.
