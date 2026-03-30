"""LambdaMART ranker for CineMatch Stage-2 re-ranking.

Loads a pre-trained LightGBM LambdaMART model and scores candidates using
the same feature set used during training (eval/build_training_data.py).
"""

import math
import os
from pathlib import Path

import lightgbm as lgb
import numpy as np

from models import CandidateMovie, RankRequest, RankedMovie, RankResponse, UserFeatures

MODEL_VERSION = "lambdamart-v1"

# Feature engineering must match eval/build_training_data.py exactly
_POPULARITY_LOG_CEIL = math.log1p(3000.0)


def _build_feature_vector(
    candidate: CandidateMovie, user: UserFeatures, user_stats: dict
) -> list[float]:
    """Build the 10-feature vector matching FEATURE_COLUMNS from training.

    Feature order: affinity_score, vote_average, log_popularity, runtime_hours,
    decade, genre_count, is_recent, user_avg_affinity, user_like_ratio,
    user_interaction_count.
    """
    # affinity_score: approximate genre affinity from user preferred genres
    preferred_set = {g.lower() for g in user.preferred_genres} if user.preferred_genres else set()
    if candidate.genres and preferred_set:
        affinity = sum(1.0 if g.lower() in preferred_set else -0.3 for g in candidate.genres) / len(candidate.genres)
    else:
        affinity = 0.0
    # Add quality boost like training data
    affinity += (candidate.vote_average - 5.0) / 10.0

    log_pop = min(math.log1p(max(candidate.popularity, 0.0)), _POPULARITY_LOG_CEIL)
    runtime_hours = (candidate.runtime or 120) / 60.0
    decade = max(0, (candidate.release_year - 1970) // 10)
    genre_count = len(candidate.genres) if candidate.genres else 0
    is_recent = 1 if candidate.release_year >= 2021 else 0

    return [
        affinity,                                  # affinity_score
        candidate.vote_average,                    # vote_average
        log_pop,                                   # log_popularity
        runtime_hours,                             # runtime_hours
        decade,                                    # decade
        genre_count,                               # genre_count
        is_recent,                                 # is_recent
        user_stats.get("user_avg_affinity", 0.0),  # user_avg_affinity
        user_stats.get("user_like_ratio", 0.5),    # user_like_ratio
        user_stats.get("user_interaction_count", 0),  # user_interaction_count
    ]


_booster: lgb.Booster | None = None


def load_model(model_path: str | None = None) -> lgb.Booster:
    """Load the LambdaMART model from disk. Caches on first call."""
    global _booster
    if _booster is not None:
        return _booster

    if model_path is None:
        model_path = os.environ.get(
            "LAMBDAMART_MODEL_PATH",
            str(Path(__file__).resolve().parent.parent / "eval" / "models" / "lambdamart-v1.txt"),
        )

    _booster = lgb.Booster(model_file=model_path)
    return _booster


def rank(request: RankRequest) -> RankResponse:
    """Re-rank candidates using the LambdaMART model."""
    booster = load_model()

    # Derive user-level stats from the request context.
    # In production these would come from the Go backend; for now we derive
    # reasonable defaults from the request itself.
    user_stats = {
        "user_avg_affinity": 0.0,
        "user_like_ratio": 0.5,
        "user_interaction_count": 0,
    }

    features = np.array([
        _build_feature_vector(c, request.user_features, user_stats)
        for c in request.candidates
    ])

    scores = booster.predict(features)

    scored = list(zip(request.candidates, scores))
    scored.sort(key=lambda x: x[1], reverse=True)

    top = scored[: request.top_n]
    ranked = [
        RankedMovie(movie_id=c.movie_id, score=round(float(s), 6), rank=i + 1)
        for i, (c, s) in enumerate(top)
    ]

    return RankResponse(ranked=ranked, model_version=MODEL_VERSION)
