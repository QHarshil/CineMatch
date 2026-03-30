"""Offline evaluation comparing feature-linear-v1 vs lambdamart-v1.

For each test user, both rankers score the user's candidate movies. We then
compute MRR, NDCG@10, and Hit Rate@10 against the held-out relevance labels.

Output: eval/results/eval_report.json + console summary
"""

import json
import sys
from pathlib import Path

import httpx
import numpy as np
import pandas as pd

RANKER_URL = "http://localhost:8000"
RESULTS_DIR = Path(__file__).resolve().parent / "results"
DATA_DIR = Path(__file__).resolve().parent / "data"

MODELS = ["feature-linear-v1", "lambdamart-v1"]
K = 10  # evaluation cutoff


def load_test_data() -> pd.DataFrame:
    """Load the held-out test split."""
    return pd.read_parquet(DATA_DIR / "test.parquet")


def build_rank_payload(user_group: pd.DataFrame, model: str) -> dict:
    """Build a /rank request payload from a user's interaction rows."""
    candidates = []
    for _, row in user_group.iterrows():
        candidates.append({
            "movie_id": row["movie_id"],
            "title": row["movie_title"],
            "genres": row["movie_genres"] if isinstance(row["movie_genres"], list) else [],
            "release_year": int(row["release_year"]),
            "vote_average": float(row["vote_average"]),
            "popularity": float(row["popularity"]),
            "runtime": int(row["runtime"]) if pd.notna(row["runtime"]) else 120,
            "similarity": max(0.0, min(1.0, float(row["affinity_score"] + 1.0) / 2.0)),
        })

    return {
        "candidates": candidates,
        "user_features": {"preferred_genres": [], "min_vote_preference": 0.0},
        "top_n": min(K, len(candidates)),
        "model": model,
    }


def call_ranker(payload: dict) -> list[str]:
    """Call the ranker service and return ranked movie_ids."""
    resp = httpx.post(f"{RANKER_URL}/rank", json=payload, timeout=10.0)
    resp.raise_for_status()
    return [r["movie_id"] for r in resp.json()["ranked"]]


def compute_ndcg_at_k(ranked_ids: list[str], relevance_map: dict[str, int], k: int) -> float:
    """Compute NDCG@k given ranked movie_ids and true relevance labels."""
    gains = [relevance_map.get(mid, 0) for mid in ranked_ids[:k]]
    dcg = sum(g / np.log2(i + 2) for i, g in enumerate(gains))

    ideal_gains = sorted(relevance_map.values(), reverse=True)[:k]
    idcg = sum(g / np.log2(i + 2) for i, g in enumerate(ideal_gains))

    return dcg / idcg if idcg > 0 else 0.0


def compute_mrr(ranked_ids: list[str], relevant_ids: set[str]) -> float:
    """Mean Reciprocal Rank: 1/rank of first relevant item."""
    for i, mid in enumerate(ranked_ids):
        if mid in relevant_ids:
            return 1.0 / (i + 1)
    return 0.0


def compute_hit_rate(ranked_ids: list[str], relevant_ids: set[str], k: int) -> float:
    """Hit Rate@k: 1 if any of the top-k results are relevant, else 0."""
    return 1.0 if any(mid in relevant_ids for mid in ranked_ids[:k]) else 0.0


def evaluate_offline(test_df: pd.DataFrame) -> dict:
    """Run offline eval without calling the ranker service.

    Scores candidates locally using both models, avoiding the need for
    a running ranker service during eval.
    """
    # Import rankers directly
    sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "ranker"))
    import ranker as linear_ranker
    import lambdamart_ranker
    from models import RankRequest

    results = {}

    for model_name in MODELS:
        ndcg_scores = []
        mrr_scores = []
        hit_rates = []

        for user_id, group in test_df.groupby("user_id"):
            if len(group) < 3:
                continue

            # Build relevance map from ground truth
            relevance_map = dict(zip(group["movie_id"], group["relevance"]))
            # Relevant = liked or watched (relevance >= 2)
            relevant_ids = {mid for mid, rel in relevance_map.items() if rel >= 2}

            if not relevant_ids:
                continue

            # Build request
            payload = build_rank_payload(group, model_name)
            request = RankRequest(**payload)

            # Score with appropriate ranker
            if model_name == "lambdamart-v1":
                response = lambdamart_ranker.rank(request)
            else:
                response = linear_ranker.rank(request)

            ranked_ids = [r.movie_id for r in response.ranked]

            ndcg_scores.append(compute_ndcg_at_k(ranked_ids, relevance_map, K))
            mrr_scores.append(compute_mrr(ranked_ids, relevant_ids))
            hit_rates.append(compute_hit_rate(ranked_ids, relevant_ids, K))

        results[model_name] = {
            "ndcg@10": float(np.mean(ndcg_scores)),
            "mrr": float(np.mean(mrr_scores)),
            "hit_rate@10": float(np.mean(hit_rates)),
            "num_users": len(ndcg_scores),
        }

    return results


def main():
    print("Loading test data...")
    test_df = load_test_data()
    print(f"  {len(test_df)} interactions, {test_df['user_id'].nunique()} users")

    print("\nRunning offline evaluation...")
    results = evaluate_offline(test_df)

    # Print comparison table
    print(f"\n{'Model':<25} {'NDCG@10':>10} {'MRR':>10} {'Hit Rate@10':>12} {'Users':>8}")
    print("-" * 68)
    for model_name, metrics in results.items():
        print(
            f"{model_name:<25} "
            f"{metrics['ndcg@10']:>10.4f} "
            f"{metrics['mrr']:>10.4f} "
            f"{metrics['hit_rate@10']:>12.4f} "
            f"{metrics['num_users']:>8}"
        )

    # Delta
    if len(results) == 2:
        m1, m2 = list(results.values())
        print(f"\n{'Delta (LM - Linear)':<25} "
              f"{m2['ndcg@10'] - m1['ndcg@10']:>+10.4f} "
              f"{m2['mrr'] - m1['mrr']:>+10.4f} "
              f"{m2['hit_rate@10'] - m1['hit_rate@10']:>+12.4f}")

    # Save
    RESULTS_DIR.mkdir(exist_ok=True)
    report_path = RESULTS_DIR / "eval_report.json"
    report_path.write_text(json.dumps(results, indent=2))
    print(f"\nReport saved to {report_path}")


if __name__ == "__main__":
    main()
