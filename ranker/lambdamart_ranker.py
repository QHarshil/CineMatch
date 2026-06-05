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

def _build_feature_vector(candidate: CandidateMovie, user: UserFeatures) -> list[float]:
    """Build the feature vector matching FEATURE_COLUMNS in
    eval/build_training_data.py. Order: similarity, vote_average,
    log_popularity, decade, is_recent, user_like_ratio, user_interaction_count.

    similarity is the pgvector cosine score from Stage-1 retrieval, the
    production analogue of the synthetic genre affinity used in training. The
    log_popularity transform matches training (np.log1p, no clamp).
    """
    log_pop = math.log1p(max(candidate.popularity, 0.0))
    decade = max(0, (candidate.release_year - 1970) // 10)
    is_recent = 1 if candidate.release_year >= 2021 else 0

    return [
        candidate.similarity,
        candidate.vote_average,
        log_pop,
        float(decade),
        float(is_recent),
        user.user_like_ratio,
        float(user.user_interaction_count),
    ]


_booster: lgb.Booster | None = None


def _resolve_model_path(model_path: str | None) -> str:
    """Locate the model file.

    Search order: explicit argument, LAMBDAMART_MODEL_PATH env var, the copy
    bundled inside the deployed image (ranker/model/), then the eval training
    output (eval/models/) for local development. Bundling the model in the
    ranker directory is what makes it available inside the container, since the
    Docker build context is the ranker/ directory.
    """
    if model_path:
        return model_path
    env_path = os.environ.get("LAMBDAMART_MODEL_PATH")
    if env_path:
        return env_path

    here = Path(__file__).resolve().parent
    candidates = [
        here / "model" / "lambdamart-v1.txt",                   # bundled in the container image
        here.parent / "eval" / "models" / "lambdamart-v1.txt",  # local dev / training output
    ]
    for candidate in candidates:
        if candidate.exists():
            return str(candidate)
    # Nothing found — return the bundled path so the loader raises a clear error.
    return str(candidates[0])


def load_model(model_path: str | None = None) -> lgb.Booster:
    """Load the LambdaMART model from disk. Caches on first call."""
    global _booster
    if _booster is not None:
        return _booster

    _booster = lgb.Booster(model_file=_resolve_model_path(model_path))
    return _booster


def rank(request: RankRequest) -> RankResponse:
    """Re-rank candidates using the LambdaMART model."""
    booster = load_model()

    # User-level features arrive on request.user_features from the Go backend;
    # similarity comes from each candidate's Stage-1 retrieval score.
    features = np.array([
        _build_feature_vector(c, request.user_features)
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
