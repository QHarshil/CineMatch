"""Build LambdaMART training data from synthetic interactions.

Reads synthetic_interactions.parquet, engineers features, assigns relevance
labels, and outputs a training-ready DataFrame with query groups.

Output: eval/data/train.parquet, eval/data/test.parquet
"""

from pathlib import Path

import numpy as np
import pandas as pd

SEED = 42
TEST_FRACTION = 0.2  # 20% of users held out for evaluation

# Relevance labels for LambdaMART (higher = more relevant)
RELEVANCE_MAP = {
    "like": 3,
    "watch": 2,
    "skip": 1,
    "dislike": 0,
}

DATA_DIR = Path(__file__).resolve().parent / "data"


def load_interactions() -> pd.DataFrame:
    """Load synthetic interactions parquet."""
    path = DATA_DIR / "synthetic_interactions.parquet"
    df = pd.read_parquet(path)
    print(f"Loaded {len(df)} interactions from {path.name}")
    return df


def engineer_features(df: pd.DataFrame) -> pd.DataFrame:
    """Engineer ranking features computed identically here and in the live
    ranker (ranker/lambdamart_ranker.py).

    Every feature must be derivable at serve time from a retrieved candidate plus
    the per-user signals the Go backend sends, so the model sees the same inputs
    in production as in training (no train/serve skew). "similarity" is the
    retrieval score: the synthetic genre affinity here, the pgvector cosine
    similarity in production. Runtime is excluded because the catalog has no
    runtime data, and raw genre affinity is folded into "similarity".
    """
    out = df.copy()

    out["similarity"] = (
        (pd.to_numeric(out["affinity_score"], errors="coerce").fillna(0.0) + 1.0) / 2.0
    ).clip(0.0, 1.0)
    out["vote_average"] = pd.to_numeric(out["vote_average"], errors="coerce").fillna(6.5)
    out["log_popularity"] = np.log1p(pd.to_numeric(out["popularity"], errors="coerce").fillna(0.0))
    out["decade"] = (
        (pd.to_numeric(out["release_year"], errors="coerce").fillna(2000) - 1970) / 10
    ).clip(lower=0).astype(int)
    out["is_recent"] = (pd.to_numeric(out["release_year"], errors="coerce").fillna(0) >= 2021).astype(int)

    # Per-user behavioural signals the Go backend can compute from interactions.
    user_stats = out.groupby("user_id").agg(
        user_like_ratio=("type", lambda x: (x == "like").mean()),
        user_interaction_count=("type", "count"),
    ).reset_index()
    out = out.merge(user_stats, on="user_id", how="left")

    out["relevance"] = out["type"].map(RELEVANCE_MAP)
    return out


FEATURE_COLUMNS = [
    "similarity",
    "vote_average",
    "log_popularity",
    "decade",
    "is_recent",
    "user_like_ratio",
    "user_interaction_count",
]


def split_by_user(df: pd.DataFrame) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Split into train/test by user (no user leakage)."""
    rng = np.random.default_rng(SEED)
    users = df["user_id"].unique()
    rng.shuffle(users)

    split_idx = int(len(users) * (1 - TEST_FRACTION))
    train_users = set(users[:split_idx])
    test_users = set(users[split_idx:])

    train_df = df[df["user_id"].isin(train_users)].copy()
    test_df = df[df["user_id"].isin(test_users)].copy()

    print(f"Train: {len(train_df)} interactions, {len(train_users)} users")
    print(f"Test:  {len(test_df)} interactions, {len(test_users)} users")
    return train_df, test_df


def main():
    interactions = load_interactions()
    featured = engineer_features(interactions)

    print(f"\nFeature columns: {FEATURE_COLUMNS}")
    print(f"Relevance distribution:\n{featured['relevance'].value_counts().sort_index().to_string()}")

    train_df, test_df = split_by_user(featured)

    train_path = DATA_DIR / "train.parquet"
    test_path = DATA_DIR / "test.parquet"
    train_df.to_parquet(train_path, index=False)
    test_df.to_parquet(test_path, index=False)
    print(f"\nSaved train to {train_path}")
    print(f"Saved test to {test_path}")


if __name__ == "__main__":
    main()
