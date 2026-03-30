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
    """Create feature columns for ranking model.

    Features:
      - affinity_score: pre-computed genre affinity (from generation)
      - vote_average: TMDB average rating
      - log_popularity: log1p of TMDB popularity score
      - runtime_hours: runtime in hours (normalized scale)
      - decade: release decade as ordinal (1970=0, 1980=1, ...)
      - genre_count: number of genres on the movie
      - is_recent: 1 if released in last 5 years, else 0
      - user_avg_affinity: mean affinity of all this user's interactions
      - user_like_ratio: fraction of user's interactions that are "like"
      - user_interaction_count: total interactions for this user
    """
    out = df.copy()

    # Basic movie features
    out["log_popularity"] = np.log1p(out["popularity"].fillna(0))
    out["runtime_hours"] = out["runtime"].fillna(120) / 60.0
    min_decade = 1970
    out["decade"] = ((out["release_year"].fillna(2000) - min_decade) / 10).clip(lower=0).astype(int)
    out["genre_count"] = out["movie_genres"].apply(lambda g: len(g) if isinstance(g, list) else 0)
    out["is_recent"] = (out["release_year"].fillna(0) >= 2021).astype(int)

    # User-level aggregate features (computed per user, then joined back)
    user_stats = out.groupby("user_id").agg(
        user_avg_affinity=("affinity_score", "mean"),
        user_like_ratio=("type", lambda x: (x == "like").mean()),
        user_interaction_count=("type", "count"),
    ).reset_index()

    out = out.merge(user_stats, on="user_id", how="left")

    # Relevance label
    out["relevance"] = out["type"].map(RELEVANCE_MAP)

    return out


FEATURE_COLUMNS = [
    "affinity_score",
    "vote_average",
    "log_popularity",
    "runtime_hours",
    "decade",
    "genre_count",
    "is_recent",
    "user_avg_affinity",
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
