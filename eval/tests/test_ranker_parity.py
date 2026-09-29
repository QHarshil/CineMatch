"""Parity between the offline eval and the live ranker.

The eval builds features and linear scores from synthetic interactions, and the
ranker builds them at serve time from a request. These tests run both on the
same rows, so a change to one side that the other does not share fails here.
"""

import sys
from pathlib import Path

import numpy as np
import pandas as pd

EVAL_DIR = Path(__file__).resolve().parent.parent
RANKER_DIR = EVAL_DIR.parent / "ranker"
sys.path.insert(0, str(EVAL_DIR))
# The ranker's modules (models, ranker) must win over eval's names.
sys.path.insert(0, str(RANKER_DIR))

import lambdamart_ranker  # noqa: E402
import ranker as linear_ranker  # noqa: E402
from build_training_data import FEATURE_COLUMNS, engineer_features  # noqa: E402
from eval_rankers import score_linear  # noqa: E402
from models import CandidateMovie, RankRequest, UserFeatures  # noqa: E402

ROWS = pd.DataFrame(
    {
        "user_id": ["u1", "u1", "u1", "u2", "u2"],
        "movie_id": ["m1", "m2", "m3", "m4", "m5"],
        "affinity_score": [0.8, -0.2, 0.1, 1.0, -1.0],
        "vote_average": [7.9, 5.5, 8.4, 6.1, 3.0],
        "popularity": [90.0, 0.0, 2500.0, 12.0, 4000.0],
        "release_year": [2016, 1965, 2021, 1999, 2024],
        "type": ["like", "skip", "watch", "like", "dislike"],
    }
)


def candidate(row: pd.Series, similarity: float) -> CandidateMovie:
    return CandidateMovie(
        movie_id=row["movie_id"],
        title=row["movie_id"],
        genres=["Drama"],
        release_year=int(row["release_year"]),
        vote_average=float(row["vote_average"]),
        popularity=float(row["popularity"]),
        runtime=100,
        similarity=similarity,
    )


def test_feature_columns_match_the_ranker_order():
    assert FEATURE_COLUMNS == lambdamart_ranker.FEATURE_ORDER


def test_serve_time_features_match_training_features():
    engineered = engineer_features(ROWS)
    for _, row in engineered.iterrows():
        user = UserFeatures(
            user_like_ratio=float(row["user_like_ratio"]),
            user_interaction_count=int(row["user_interaction_count"]),
        )
        served = lambdamart_ranker._build_feature_vector(
            candidate(row, float(row["similarity"])), user
        )
        trained = row[FEATURE_COLUMNS].to_numpy(dtype=float)
        assert np.allclose(served, trained), (row["movie_id"], served, trained)


def test_eval_linear_scores_match_the_live_linear_ranker():
    offline = dict(zip(ROWS["movie_id"], score_linear(ROWS)))
    similarity = np.clip((ROWS["affinity_score"] + 1.0) / 2.0, 0.0, 1.0)
    request = RankRequest(
        candidates=[
            candidate(row, float(sim))
            for (_, row), sim in zip(ROWS.iterrows(), similarity)
        ],
        top_n=len(ROWS),
        model=linear_ranker.MODEL_VERSION,
    )
    live = {r.movie_id: r.score for r in linear_ranker.rank(request).ranked}
    assert live.keys() == offline.keys()
    for movie_id, score in live.items():
        assert abs(score - offline[movie_id]) < 1e-6, (
            movie_id,
            score,
            offline[movie_id],
        )
