"""Train LambdaMART model on synthetic interaction data.

Reads train.parquet, trains a LightGBM LambdaMART ranker grouped by user_id,
evaluates on test.parquet, and saves the model.

Output: eval/models/lambdamart-v1.txt (LightGBM booster)
"""

import json
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pandas as pd
from sklearn.metrics import ndcg_score

from build_training_data import FEATURE_COLUMNS

SEED = 42
DATA_DIR = Path(__file__).resolve().parent / "data"
MODEL_DIR = Path(__file__).resolve().parent / "models"


def load_splits() -> tuple[pd.DataFrame, pd.DataFrame]:
    """Load train and test parquet files."""
    train = pd.read_parquet(DATA_DIR / "train.parquet")
    test = pd.read_parquet(DATA_DIR / "test.parquet")
    return train, test


def build_query_groups(df: pd.DataFrame) -> np.ndarray:
    """Build LightGBM group array: number of items per query (user)."""
    return df.groupby("user_id", sort=False).size().values


def train_ranker(train_df: pd.DataFrame) -> lgb.Booster:
    """Train LambdaMART ranker using LightGBM."""
    X_train = train_df[FEATURE_COLUMNS].values
    y_train = train_df["relevance"].values
    groups = build_query_groups(train_df)

    train_data = lgb.Dataset(
        X_train,
        label=y_train,
        group=groups,
        feature_name=FEATURE_COLUMNS,
        free_raw_data=False,
    )

    params = {
        "objective": "lambdarank",
        "metric": "ndcg",
        "eval_at": [5, 10],
        "learning_rate": 0.05,
        "num_leaves": 31,
        "min_data_in_leaf": 10,
        "max_depth": 6,
        "feature_fraction": 0.8,
        "bagging_fraction": 0.8,
        "bagging_freq": 5,
        "lambdarank_truncation_level": 10,
        "verbose": -1,
        "seed": SEED,
    }

    booster = lgb.train(
        params,
        train_data,
        num_boost_round=200,
        valid_sets=[train_data],
        valid_names=["train"],
    )

    return booster


def evaluate_ranker(booster: lgb.Booster, test_df: pd.DataFrame) -> dict:
    """Evaluate on test set: NDCG@5, NDCG@10 per user, then average."""
    X_test = test_df[FEATURE_COLUMNS].values
    scores = booster.predict(X_test)
    test_df = test_df.copy()
    test_df["pred_score"] = scores

    ndcg_5_list = []
    ndcg_10_list = []

    for _, group in test_df.groupby("user_id"):
        if len(group) < 2:
            continue
        true_rel = group["relevance"].values.reshape(1, -1)
        pred_scores = group["pred_score"].values.reshape(1, -1)

        ndcg_5_list.append(ndcg_score(true_rel, pred_scores, k=5))
        ndcg_10_list.append(ndcg_score(true_rel, pred_scores, k=10))

    return {
        "ndcg@5": float(np.mean(ndcg_5_list)),
        "ndcg@10": float(np.mean(ndcg_10_list)),
        "num_test_users": len(ndcg_5_list),
    }


def main():
    print("Loading train/test splits...")
    train_df, test_df = load_splits()
    print(f"Train: {len(train_df)} rows, Test: {len(test_df)} rows")

    print("\nTraining LambdaMART...")
    booster = train_ranker(train_df)

    # Feature importance
    importance = booster.feature_importance(importance_type="gain")
    feat_imp = sorted(
        zip(FEATURE_COLUMNS, importance), key=lambda x: x[1], reverse=True
    )
    print("\nFeature importance (gain):")
    for name, gain in feat_imp:
        print(f"  {name:30s} {gain:.1f}")

    print("\nEvaluating on test set...")
    metrics = evaluate_ranker(booster, test_df)
    print(f"  NDCG@5:  {metrics['ndcg@5']:.4f}")
    print(f"  NDCG@10: {metrics['ndcg@10']:.4f}")
    print(f"  Test users: {metrics['num_test_users']}")

    # Save model
    MODEL_DIR.mkdir(exist_ok=True)
    model_path = MODEL_DIR / "lambdamart-v1.txt"
    booster.save_model(str(model_path))
    print(f"\nModel saved to {model_path}")

    # Save metadata
    meta = {
        "model_version": "lambdamart-v1",
        "features": FEATURE_COLUMNS,
        "num_boost_round": 200,
        "train_rows": len(train_df),
        "test_rows": len(test_df),
        "metrics": metrics,
        "feature_importance": {name: float(gain) for name, gain in feat_imp},
    }
    meta_path = MODEL_DIR / "lambdamart-v1-meta.json"
    meta_path.write_text(json.dumps(meta, indent=2))
    print(f"Metadata saved to {meta_path}")


if __name__ == "__main__":
    main()
