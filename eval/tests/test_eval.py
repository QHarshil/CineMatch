"""Tests for eval pipeline: metrics, feature engineering, training data."""

import numpy as np
import pandas as pd
import pytest

import sys
from pathlib import Path

# Add eval directory to path
EVAL_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVAL_DIR))

from eval_rankers import compute_ndcg_at_k, compute_mrr, compute_hit_rate
from build_training_data import engineer_features, RELEVANCE_MAP, FEATURE_COLUMNS


# ── Metric tests ──────────────────────────────────────────────────────────────

class TestNDCG:
    def test_perfect_ranking(self):
        """NDCG@3 should be 1.0 when items are in ideal order."""
        ranked = ["a", "b", "c"]
        relevance = {"a": 3, "b": 2, "c": 1}
        assert compute_ndcg_at_k(ranked, relevance, 3) == pytest.approx(1.0)

    def test_reversed_ranking(self):
        """NDCG should be < 1.0 when items are in worst order."""
        ranked = ["c", "b", "a"]
        relevance = {"a": 3, "b": 2, "c": 1}
        assert compute_ndcg_at_k(ranked, relevance, 3) < 1.0

    def test_single_item(self):
        ranked = ["a"]
        relevance = {"a": 3}
        assert compute_ndcg_at_k(ranked, relevance, 1) == pytest.approx(1.0)

    def test_no_relevant_items(self):
        ranked = ["a", "b"]
        relevance = {"a": 0, "b": 0}
        assert compute_ndcg_at_k(ranked, relevance, 2) == pytest.approx(0.0)

    def test_k_truncation(self):
        """Only top-k items should matter."""
        ranked = ["bad", "a"]
        relevance = {"a": 3, "bad": 0}
        ndcg_1 = compute_ndcg_at_k(ranked, relevance, 1)
        assert ndcg_1 == pytest.approx(0.0)  # "bad" is at position 1, relevance=0


class TestMRR:
    def test_first_item_relevant(self):
        assert compute_mrr(["a", "b"], {"a"}) == pytest.approx(1.0)

    def test_second_item_relevant(self):
        assert compute_mrr(["b", "a"], {"a"}) == pytest.approx(0.5)

    def test_no_relevant_items(self):
        assert compute_mrr(["b", "c"], {"a"}) == pytest.approx(0.0)


class TestHitRate:
    def test_hit_in_top_k(self):
        assert compute_hit_rate(["a", "b", "c"], {"b"}, 3) == pytest.approx(1.0)

    def test_no_hit(self):
        assert compute_hit_rate(["a", "b", "c"], {"d"}, 3) == pytest.approx(0.0)

    def test_hit_beyond_k(self):
        assert compute_hit_rate(["a", "b", "c"], {"c"}, 2) == pytest.approx(0.0)


# ── Feature engineering tests ─────────────────────────────────────────────────

class TestFeatureEngineering:
    @pytest.fixture()
    def sample_interactions(self):
        return pd.DataFrame({
            "user_id": ["u1", "u1", "u1", "u2", "u2"],
            "movie_id": ["m1", "m2", "m3", "m1", "m4"],
            "type": ["like", "watch", "skip", "dislike", "like"],
            "affinity_score": [0.8, 0.3, -0.2, -0.5, 0.6],
            "movie_genres": [["Action"], ["Drama"], ["Comedy"], ["Horror"], ["Action"]],
            "vote_average": [8.0, 7.0, 5.0, 4.0, 7.5],
            "popularity": [500.0, 200.0, 50.0, 10.0, 300.0],
            "release_year": [2022, 2015, 2000, 1995, 2023],
            "runtime": [120, 90, 110, 85, 130],
            "profile": ["action_fan"] * 3 + ["horror_buff"] * 2,
            "movie_title": ["M1", "M2", "M3", "M1", "M4"],
        })

    def test_relevance_labels(self, sample_interactions):
        featured = engineer_features(sample_interactions)
        expected = [RELEVANCE_MAP["like"], RELEVANCE_MAP["watch"],
                    RELEVANCE_MAP["skip"], RELEVANCE_MAP["dislike"], RELEVANCE_MAP["like"]]
        assert featured["relevance"].tolist() == expected

    def test_all_feature_columns_present(self, sample_interactions):
        featured = engineer_features(sample_interactions)
        for col in FEATURE_COLUMNS:
            assert col in featured.columns, f"Missing feature column: {col}"

    def test_is_recent_flag(self, sample_interactions):
        featured = engineer_features(sample_interactions)
        # 2022 and 2023 are recent (>= 2021), others are not
        assert featured["is_recent"].tolist() == [1, 0, 0, 0, 1]

    def test_user_interaction_count(self, sample_interactions):
        featured = engineer_features(sample_interactions)
        u1_rows = featured[featured["user_id"] == "u1"]
        assert (u1_rows["user_interaction_count"] == 3).all()

    def test_log_popularity_positive(self, sample_interactions):
        featured = engineer_features(sample_interactions)
        assert (featured["log_popularity"] > 0).all()


# ── Data integrity tests ─────────────────────────────────────────────────────

class TestDataIntegrity:
    def test_parquet_files_exist(self):
        data_dir = EVAL_DIR / "data"
        assert (data_dir / "synthetic_interactions.parquet").exists()
        assert (data_dir / "train.parquet").exists()
        assert (data_dir / "test.parquet").exists()

    def test_no_user_leakage(self):
        """Train and test sets should have disjoint user sets."""
        train = pd.read_parquet(EVAL_DIR / "data" / "train.parquet")
        test = pd.read_parquet(EVAL_DIR / "data" / "test.parquet")
        train_users = set(train["user_id"].unique())
        test_users = set(test["user_id"].unique())
        assert train_users.isdisjoint(test_users), "User leakage between train/test"

    def test_relevance_values_valid(self):
        train = pd.read_parquet(EVAL_DIR / "data" / "train.parquet")
        assert set(train["relevance"].unique()).issubset({0, 1, 2, 3})

    def test_model_file_exists(self):
        model_path = EVAL_DIR / "models" / "lambdamart-v1.txt"
        assert model_path.exists(), "LambdaMART model file not found"
