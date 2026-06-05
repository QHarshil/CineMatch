"""Offline evaluation of the recommendation stack on held-out synthetic users.

Compares four rankers on the same held-out (user, movie) rows:

  - popularity      : rank by TMDB popularity only (no personalization)
  - vector_only     : rank by retrieval similarity only (Stage-1 with no re-rank)
  - feature-linear-v1: the explicit weighted scorer (Stage-2 linear)
  - lambdamart-v1   : the trained LightGBM model (Stage-2 learned)

Features for the learned model are engineered identically to training
(build_training_data.FEATURE_COLUMNS) so the comparison is apples-to-apples and
free of train/serve skew. Relevance is graded: like=3, watch=2, skip=1,
dislike=0; an item is "relevant" for MRR / Hit Rate when relevance >= 2.

Output: eval/results/eval_report.json + a console table.
"""

import json
import math
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pandas as pd

from build_training_data import FEATURE_COLUMNS

K = 10
MIN_GROUP = 3  # users need at least this many rated candidates to be scored
DATA_DIR = Path(__file__).resolve().parent / "data"
RESULTS_DIR = Path(__file__).resolve().parent / "results"
MODEL_DIR = Path(__file__).resolve().parent / "models"

# Linear scorer weights — kept in sync with ranker/ranker.py.
_W_SIM, _W_QUALITY, _W_POP, _W_GENRE = 0.50, 0.25, 0.15, 0.10
_POP_LOG_CEIL = math.log1p(3000.0)


def score_popularity(group: pd.DataFrame) -> np.ndarray:
    return group["popularity"].to_numpy(dtype=float)


def score_vector_only(group: pd.DataFrame) -> np.ndarray:
    """Stage-1 only: rank by retrieval similarity (the genre affinity signal)."""
    return group["affinity_score"].to_numpy(dtype=float)


def score_linear(group: pd.DataFrame) -> np.ndarray:
    """The feature-linear-v1 formula, evaluated row-wise on the test frame."""
    similarity = np.clip((group["affinity_score"].to_numpy(dtype=float) + 1.0) / 2.0, 0.0, 1.0)
    quality = group["vote_average"].to_numpy(dtype=float) / 10.0
    pop = group["popularity"].to_numpy(dtype=float)
    log_pop = np.minimum(np.log1p(np.clip(pop, 0, None)) / _POP_LOG_CEIL, 1.0)
    # No stated genre preferences in the offline payload, so genre overlap is
    # the neutral 0.5 the live ranker also uses in that case.
    return _W_SIM * similarity + _W_QUALITY * quality + _W_POP * log_pop + _W_GENRE * 0.5


def make_lambdamart_scorer(booster: lgb.Booster):
    def score(group: pd.DataFrame) -> np.ndarray:
        return booster.predict(group[FEATURE_COLUMNS].to_numpy(dtype=float))
    return score


def ndcg_at_k(order_relevance: np.ndarray, k: int) -> float:
    gains = order_relevance[:k]
    dcg = np.sum(gains / np.log2(np.arange(2, len(gains) + 2)))
    ideal = np.sort(order_relevance)[::-1][:k]
    idcg = np.sum(ideal / np.log2(np.arange(2, len(ideal) + 2)))
    return float(dcg / idcg) if idcg > 0 else 0.0


def mrr(order_relevance: np.ndarray) -> float:
    relevant = np.where(order_relevance >= 2)[0]
    return float(1.0 / (relevant[0] + 1)) if relevant.size else 0.0


def hit_rate_at_k(order_relevance: np.ndarray, k: int) -> float:
    return 1.0 if np.any(order_relevance[:k] >= 2) else 0.0


def evaluate(test_df: pd.DataFrame, scorers: dict) -> dict:
    results = {name: {"ndcg": [], "mrr": [], "hit": []} for name in scorers}
    n_users = 0
    for _, group in test_df.groupby("user_id"):
        relevance = group["relevance"].to_numpy(dtype=float)
        if len(group) < MIN_GROUP or not np.any(relevance >= 2):
            continue
        n_users += 1
        for name, scorer in scorers.items():
            order = np.argsort(-scorer(group), kind="stable")
            ordered_rel = relevance[order]
            results[name]["ndcg"].append(ndcg_at_k(ordered_rel, K))
            results[name]["mrr"].append(mrr(ordered_rel))
            results[name]["hit"].append(hit_rate_at_k(ordered_rel, K))

    report = {}
    for name, m in results.items():
        report[name] = {
            "ndcg@10": round(float(np.mean(m["ndcg"])), 4),
            "mrr": round(float(np.mean(m["mrr"])), 4),
            "hit_rate@10": round(float(np.mean(m["hit"])), 4),
            "num_users": n_users,
        }
    return report


def main():
    test_df = pd.read_parquet(DATA_DIR / "test.parquet")
    booster = lgb.Booster(model_file=str(MODEL_DIR / "lambdamart-v1.txt"))

    scorers = {
        "popularity": score_popularity,
        "vector_only": score_vector_only,
        "feature-linear-v1": score_linear,
        "lambdamart-v1": make_lambdamart_scorer(booster),
    }
    report = evaluate(test_df, scorers)

    print(f"\n{'Model':<20} {'NDCG@10':>9} {'MRR':>8} {'Hit@10':>8}")
    print("-" * 47)
    for name in scorers:
        r = report[name]
        print(f"{name:<20} {r['ndcg@10']:>9.4f} {r['mrr']:>8.4f} {r['hit_rate@10']:>8.4f}")

    base = report["popularity"]["ndcg@10"]
    best = report["lambdamart-v1"]["ndcg@10"]
    lift = 100 * (best - base) / base if base else 0.0
    print(f"\nlambdamart-v1 vs popularity: NDCG@10 {base:.4f} -> {best:.4f} (+{lift:.0f}%)")

    RESULTS_DIR.mkdir(exist_ok=True)
    (RESULTS_DIR / "eval_report.json").write_text(json.dumps(report, indent=2))
    print(f"Saved {RESULTS_DIR / 'eval_report.json'}")


if __name__ == "__main__":
    main()
