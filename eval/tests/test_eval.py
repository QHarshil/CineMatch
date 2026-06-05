"""Tests for eval pipeline: metrics, feature engineering, training data."""

import numpy as np
import pandas as pd
import pytest

import sys
from pathlib import Path

# Add eval directory to path
EVAL_DIR = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(EVAL_DIR))

from eval_rankers import (
    ndcg_at_k,
    mrr,
    hit_rate_at_k,
    score_popularity,
    score_vector_only,
    score_linear,
)
from build_training_data import engineer_features, RELEVANCE_MAP, FEATURE_COLUMNS


# ── Metric tests ──────────────────────────────────────────────────────────────

class TestNDCG:
    """Metrics take relevance values already in ranked order (graded gains)."""

    def test_perfect_ranking(self):
        assert ndcg_at_k(np.array([3, 2, 1]), 3) == pytest.approx(1.0)

    def test_reversed_ranking(self):
        assert ndcg_at_k(np.array([1, 2, 3]), 3) < 1.0

    def test_single_item(self):
        assert ndcg_at_k(np.array([3]), 1) == pytest.approx(1.0)

    def test_no_relevant_items(self):
        assert ndcg_at_k(np.array([0, 0]), 2) == pytest.approx(0.0)

    def test_k_truncation(self):
        # An irrelevant item at position 1 pushes the relevant one past the cutoff.
        assert ndcg_at_k(np.array([0, 3]), 1) == pytest.approx(0.0)


class TestMRR:
    def test_first_item_relevant(self):
        assert mrr(np.array([3, 1])) == pytest.approx(1.0)

    def test_second_item_relevant(self):
        assert mrr(np.array([1, 2])) == pytest.approx(0.5)

    def test_no_relevant_items(self):
        # skip (1) and dislike (0) are below the relevance >= 2 bar.
        assert mrr(np.array([1, 0])) == pytest.approx(0.0)


class TestHitRate:
    def test_hit_in_top_k(self):
        assert hit_rate_at_k(np.array([1, 2, 1]), 3) == pytest.approx(1.0)

    def test_no_hit(self):
        assert hit_rate_at_k(np.array([1, 1, 1]), 3) == pytest.approx(0.0)

    def test_hit_beyond_k(self):
        assert hit_rate_at_k(np.array([1, 1, 2]), 2) == pytest.approx(0.0)


class TestScorers:
    @pytest.fixture()
    def group(self):
        # First row is the stronger candidate on every signal.
        return pd.DataFrame({
            "affinity_score": [0.9, -0.5],
            "vote_average": [8.0, 5.0],
            "popularity": [500.0, 20.0],
        })

    def test_vector_only_prefers_higher_affinity(self, group):
        scores = score_vector_only(group)
        assert scores[0] > scores[1]

    def test_linear_prefers_better_candidate(self, group):
        scores = score_linear(group)
        assert scores[0] > scores[1]

    def test_popularity_prefers_more_popular(self, group):
        scores = score_popularity(group)
        assert scores[0] > scores[1]


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
