"""LambdaMART ranker for CineMatch Stage-2 re-ranking.

Loads a pre-trained LightGBM LambdaMART model and scores candidates using
the same feature set used during training (eval/build_training_data.py).
"""

import json
import math
import os
from dataclasses import dataclass
from pathlib import Path

import lightgbm as lgb
import numpy as np

from models import (
    CandidateMovie,
    RankingFactor,
    RankRequest,
    RankedMovie,
    RankResponse,
    UserFeatures,
)

DEFAULT_MODEL_VERSION = "lambdamart-v1"

# Title-level columns of the feature vector, in order. The user-level columns
# that follow are the same for every candidate in a request, so they cannot
# explain why one title outranks another and are left out of explanations.
ITEM_FEATURES = ["similarity", "vote_average", "log_popularity", "decade", "is_recent"]
USER_FEATURES = ["user_like_ratio", "user_interaction_count"]
# The column order _build_feature_vector produces. load_model checks the model
# file was trained on exactly this order.
FEATURE_ORDER = ITEM_FEATURES + USER_FEATURES
MAX_FACTORS = 3


@dataclass(frozen=True)
class LoadedModel:
    """A trained booster and the version reported with its rankings."""

    booster: lgb.Booster
    version: str


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
        here / "model" / "lambdamart-v1.txt",  # bundled in the container image
        here.parent
        / "eval"
        / "models"
        / "lambdamart-v1.txt",  # local dev / training output
    ]
    for candidate in candidates:
        if candidate.exists():
            return str(candidate)
    # Nothing found: return the bundled path so the loader raises a clear error.
    return str(candidates[0])


def load_model(model_path: str | None = None) -> LoadedModel:
    """Load a LambdaMART model and check it expects FEATURE_ORDER.

    The version comes from the metadata file training writes beside the model
    (lambdamart-v1-meta.json), or the file name when there is none.
    """
    path = Path(_resolve_model_path(model_path))
    booster = lgb.Booster(model_file=str(path))
    if booster.feature_name() != FEATURE_ORDER:
        raise ValueError(
            f"{path.name} expects features {booster.feature_name()}, "
            f"but the ranker builds {FEATURE_ORDER}"
        )
    return LoadedModel(booster=booster, version=_model_version(path))


def _model_version(path: Path) -> str:
    meta = path.with_name(f"{path.stem}-meta.json")
    if meta.exists():
        return json.loads(meta.read_text()).get("model_version", path.stem)
    return path.stem


def rank(request: RankRequest, model: LoadedModel) -> RankResponse:
    """Score candidates with LambdaMART and attach each pick's SHAP factors."""
    booster = model.booster

    # User-level features arrive on request.user_features from the Go backend;
    # similarity comes from each candidate's Stage-1 retrieval score.
    features = np.array(
        [_build_feature_vector(c, request.user_features) for c in request.candidates]
    )

    scores = booster.predict(features)
    # SHAP values per feature; the last column is the expected-value baseline.
    contributions = booster.predict(features, pred_contrib=True)

    scored = list(zip(request.candidates, scores, contributions))
    scored.sort(key=lambda x: x[1], reverse=True)

    top = scored[: request.top_n]
    ranked = [
        RankedMovie(
            movie_id=c.movie_id,
            score=round(float(s), 6),
            rank=i + 1,
            factors=top_factors(contrib),
        )
        for i, (c, s, contrib) in enumerate(top)
    ]

    return RankResponse(ranked=ranked, model_version=model.version)


def top_factors(contributions: np.ndarray) -> list[RankingFactor]:
    """Return the title-level features that raised a score, largest first."""
    item = [
        (name, float(value))
        for name, value in zip(ITEM_FEATURES, contributions[: len(ITEM_FEATURES)])
        if value > 0
    ]
    item.sort(key=lambda pair: pair[1], reverse=True)
    return [
        RankingFactor(feature=name, contribution=round(value, 4))
        for name, value in item[:MAX_FACTORS]
    ]
