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

Run tests:

```bash
python -m pytest tests/ -v             # 20 tests
```

Output files:
- `data/synthetic_interactions.parquet` -- 9607 interactions across 200 users
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

Most recent eval on synthetic data:

| Model | NDCG@10 | MRR | Hit Rate@10 |
|-------|---------|-----|-------------|
| Popularity baseline | 0.62 | 0.71 | 0.85 |
| Vector retrieval only | 0.76 | 0.89 | 0.95 |
| Two-stage (feature-linear-v1) | 0.86 | 1.00 | 1.00 |
| Two-stage (lambdamart-v1) | 0.71 | 0.86 | 1.00 |

The linear ranker outperforms LambdaMART here, which is expected. The synthetic users were generated with a process that mirrors the linear formula (genre affinity + quality boost), so the linear model is almost perfectly aligned with the "ground truth." LambdaMART has to learn this relationship from data, and with synthetic data that's inherently circular, it can't do better.

With real user data where preferences are messier and non-linear, LambdaMART should close this gap because it can capture interactions between features that the linear formula can't express.

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

Each user generates 20-80 interactions against the real 494-movie catalog (fetched from Supabase). Genre affinity determines which movies a user interacts with and how: loved genres add +1.0, liked genres +0.4, disliked genres -0.7. Gaussian noise (sigma=0.3) prevents deterministic outcomes.

## Feature engineering

`build_training_data.py` computes 10 features per (user, movie) pair:

| Feature | Description |
|---------|-------------|
| affinity_score | Genre affinity between movie and user's taste profile |
| vote_average | TMDB average rating [0, 10] |
| log_popularity | log1p of TMDB popularity score |
| runtime_hours | Runtime in fractional hours |
| decade | Release decade as ordinal (1970s=0, 1980s=1, ...) |
| genre_count | Number of genres on the movie |
| is_recent | Binary: 1 if released >= 2021 |
| user_avg_affinity | Mean affinity across the user's interaction history |
| user_like_ratio | Fraction of user's interactions that are "like" |
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
