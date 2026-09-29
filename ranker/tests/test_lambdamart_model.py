"""Tests for loading the LambdaMART model and routing requests to it."""

import json
import shutil
import sys
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pytest
from fastapi.testclient import TestClient

sys.path.insert(0, str(Path(__file__).parent.parent))

import lambdamart_ranker  # noqa: E402
from main import app  # noqa: E402

BUNDLED_MODEL = Path(__file__).parent.parent / "model" / "lambdamart-v1.txt"


def test_bundled_model_matches_the_serve_time_features():
    model = lambdamart_ranker.load_model(str(BUNDLED_MODEL))
    assert model.booster.feature_name() == lambdamart_ranker.FEATURE_ORDER
    assert model.version == "lambdamart-v1"


def test_load_model_rejects_a_different_feature_order(tmp_path):
    rng = np.random.default_rng(0)
    features = rng.random((40, 7))
    reordered = list(reversed(lambdamart_ranker.FEATURE_ORDER))
    booster = lgb.train(
        {"objective": "regression", "verbose": -1},
        lgb.Dataset(features, label=rng.random(40), feature_name=reordered),
        num_boost_round=2,
    )
    path = tmp_path / "reordered.txt"
    booster.save_model(str(path))

    with pytest.raises(ValueError, match="expects features"):
        lambdamart_ranker.load_model(str(path))


def test_version_comes_from_the_training_metadata(tmp_path):
    path = tmp_path / "lambdamart-v2.txt"
    shutil.copy(BUNDLED_MODEL, path)
    (tmp_path / "lambdamart-v2-meta.json").write_text(
        json.dumps({"model_version": "lambdamart-v2"})
    )

    assert lambdamart_ranker.load_model(str(path)).version == "lambdamart-v2"


def test_a_request_for_another_version_uses_the_linear_ranker(tmp_path):
    path = tmp_path / "lambdamart-v2.txt"
    shutil.copy(BUNDLED_MODEL, path)
    candidate = {
        "movie_id": "cccccccc-0000-0000-0000-000000000001",
        "title": "Arrival",
        "genres": ["Drama"],
        "release_year": 2016,
        "vote_average": 7.9,
        "popularity": 90.0,
        "runtime": 116,
        "similarity": 0.8,
    }
    with TestClient(app) as client:
        app.state.lambdamart = lambdamart_ranker.load_model(str(path))
        stale = client.post(
            "/rank", json={"candidates": [candidate], "model": "lambdamart-v1"}
        )
        current = client.post(
            "/rank", json={"candidates": [candidate], "model": "lambdamart-v2"}
        )

    assert stale.json()["model_version"] == "feature-linear-v1"
    assert current.json()["model_version"] == "lambdamart-v2"
