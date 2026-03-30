"""Tests for the /rank endpoint and the underlying feature ranker."""

import sys
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

# Allow imports from the ranker package root (main.py, ranker.py, models.py).
sys.path.insert(0, str(Path(__file__).parent.parent))

from main import app  # noqa: E402
from models import CandidateMovie, RankRequest, UserFeatures  # noqa: E402
from ranker import MODEL_VERSION, _genre_overlap, _log_popularity_score, rank  # noqa: E402

client = TestClient(app)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


def make_candidate(
    movie_id: str = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    similarity: float = 0.80,
    vote_average: float = 7.5,
    popularity: float = 200.0,
    genres: list[str] | None = None,
    release_year: int = 2020,
    title: str = "Test Movie",
) -> CandidateMovie:
    return CandidateMovie(
        movie_id=movie_id,
        title=title,
        genres=genres or ["Drama"],
        release_year=release_year,
        vote_average=vote_average,
        popularity=popularity,
        runtime=120,
        similarity=similarity,
    )


CANDIDATE_A = make_candidate(movie_id="aaa-1", similarity=0.95, vote_average=8.5, genres=["Action", "Drama"])
CANDIDATE_B = make_candidate(movie_id="bbb-2", similarity=0.70, vote_average=6.0, genres=["Comedy"])
CANDIDATE_C = make_candidate(movie_id="ccc-3", similarity=0.85, vote_average=7.0, genres=["Drama", "Thriller"])


# ---------------------------------------------------------------------------
# Unit tests — scoring helpers
# ---------------------------------------------------------------------------


def test_log_popularity_score_clamps_to_one():
    assert _log_popularity_score(1_000_000) == 1.0


def test_log_popularity_score_zero_popularity():
    assert _log_popularity_score(0.0) == 0.0


def test_log_popularity_score_typical_range():
    score = _log_popularity_score(200.0)
    assert 0.0 < score < 1.0


def test_genre_overlap_full_match():
    assert _genre_overlap(["Action", "Drama"], ["Action", "Drama"]) == 1.0


def test_genre_overlap_partial_match():
    overlap = _genre_overlap(["Action", "Drama"], ["Action"])
    assert overlap == 0.5


def test_genre_overlap_no_match():
    assert _genre_overlap(["Comedy"], ["Action", "Drama"]) == 0.0


def test_genre_overlap_no_user_preferences_returns_neutral():
    # No preferences → neutral 0.5, not penalising
    assert _genre_overlap(["Action"], []) == 0.5


def test_genre_overlap_case_insensitive():
    assert _genre_overlap(["action"], ["Action"]) == 1.0


# ---------------------------------------------------------------------------
# Unit tests — rank() function
# ---------------------------------------------------------------------------


def test_rank_returns_top_n():
    request = RankRequest(candidates=[CANDIDATE_A, CANDIDATE_B, CANDIDATE_C], top_n=2)
    response = rank(request)
    assert len(response.ranked) == 2


def test_rank_returns_all_when_fewer_than_top_n():
    request = RankRequest(candidates=[CANDIDATE_A, CANDIDATE_B], top_n=20)
    response = rank(request)
    assert len(response.ranked) == 2


def test_rank_order_descending_by_score():
    request = RankRequest(candidates=[CANDIDATE_A, CANDIDATE_B, CANDIDATE_C])
    response = rank(request)
    scores = [r.score for r in response.ranked]
    assert scores == sorted(scores, reverse=True)


def test_rank_positions_are_1_indexed():
    request = RankRequest(candidates=[CANDIDATE_A, CANDIDATE_B])
    response = rank(request)
    assert response.ranked[0].rank == 1
    assert response.ranked[1].rank == 2


def test_rank_high_similarity_beats_low_similarity():
    """Candidate A (similarity=0.95) should outscore B (similarity=0.70)."""
    low_sim = make_candidate(movie_id="low", similarity=0.30, vote_average=9.0, popularity=500.0)
    high_sim = make_candidate(movie_id="high", similarity=0.95, vote_average=5.0, popularity=10.0)
    request = RankRequest(candidates=[low_sim, high_sim])
    response = rank(request)
    assert response.ranked[0].movie_id == "high"


def test_rank_genre_preference_boosts_matching_candidate():
    """With strong genre preference, the matching candidate should rank higher."""
    action = make_candidate(movie_id="action", similarity=0.75, genres=["Action"])
    comedy = make_candidate(movie_id="comedy", similarity=0.75, genres=["Comedy"])
    user = UserFeatures(preferred_genres=["Action"])
    request = RankRequest(candidates=[comedy, action], user_features=user)
    response = rank(request)
    assert response.ranked[0].movie_id == "action"


def test_rank_vote_floor_penalty_applied():
    """A movie below min_vote_preference should score lower than a comparable movie above it."""
    above = make_candidate(movie_id="above", similarity=0.80, vote_average=8.0)
    below = make_candidate(movie_id="below", similarity=0.80, vote_average=4.0)
    user = UserFeatures(min_vote_preference=7.0)
    request = RankRequest(candidates=[below, above], user_features=user)
    response = rank(request)
    assert response.ranked[0].movie_id == "above"


def test_rank_includes_model_version():
    request = RankRequest(candidates=[CANDIDATE_A])
    response = rank(request)
    assert response.model_version == MODEL_VERSION


def test_rank_single_candidate():
    request = RankRequest(candidates=[CANDIDATE_A])
    response = rank(request)
    assert len(response.ranked) == 1
    assert response.ranked[0].rank == 1


# ---------------------------------------------------------------------------
# Integration tests — HTTP endpoint via TestClient
# ---------------------------------------------------------------------------


def test_health_endpoint():
    resp = client.get("/health")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"


def test_post_rank_valid_request():
    payload = {
        "candidates": [
            {
                "movie_id": "aaaaaaaa-0000-0000-0000-000000000001",
                "title": "Inception",
                "genres": ["Action", "Science Fiction"],
                "release_year": 2010,
                "vote_average": 8.8,
                "popularity": 850.0,
                "runtime": 148,
                "similarity": 0.92,
            },
            {
                "movie_id": "aaaaaaaa-0000-0000-0000-000000000002",
                "title": "Paul Blart: Mall Cop",
                "genres": ["Comedy"],
                "release_year": 2009,
                "vote_average": 5.5,
                "popularity": 60.0,
                "runtime": 91,
                "similarity": 0.55,
            },
        ],
        "user_features": {"preferred_genres": ["Action"], "min_vote_preference": 6.0},
        "top_n": 5,
    }
    resp = client.post("/rank", json=payload)
    assert resp.status_code == 200
    body = resp.json()
    assert len(body["ranked"]) == 2
    assert body["ranked"][0]["movie_id"] == "aaaaaaaa-0000-0000-0000-000000000001"
    assert body["model_version"] == MODEL_VERSION


def test_post_rank_respects_top_n():
    candidates = [
        {
            "movie_id": f"aaaaaaaa-0000-0000-0000-{i:012d}",
            "title": f"Movie {i}",
            "genres": ["Drama"],
            "release_year": 2020,
            "vote_average": 7.0,
            "popularity": float(100 + i),
            "runtime": 100,
            "similarity": round(0.9 - i * 0.01, 2),
        }
        for i in range(10)
    ]
    resp = client.post("/rank", json={"candidates": candidates, "top_n": 3})
    assert resp.status_code == 200
    assert len(resp.json()["ranked"]) == 3


def test_post_rank_rejects_empty_candidates():
    resp = client.post("/rank", json={"candidates": []})
    assert resp.status_code == 422


def test_post_rank_rejects_top_n_above_max():
    candidate = {
        "movie_id": "aaaaaaaa-0000-0000-0000-000000000001",
        "title": "Movie",
        "genres": ["Drama"],
        "release_year": 2020,
        "vote_average": 7.0,
        "popularity": 100.0,
        "runtime": 100,
        "similarity": 0.8,
    }
    resp = client.post("/rank", json={"candidates": [candidate], "top_n": 51})
    assert resp.status_code == 422


def test_post_rank_default_user_features():
    """Endpoint works with no user_features supplied — defaults to neutral scoring."""
    candidate = {
        "movie_id": "aaaaaaaa-0000-0000-0000-000000000001",
        "title": "Movie",
        "genres": ["Drama"],
        "release_year": 2020,
        "vote_average": 7.0,
        "popularity": 100.0,
        "runtime": 100,
        "similarity": 0.8,
    }
    resp = client.post("/rank", json={"candidates": [candidate]})
    assert resp.status_code == 200
    assert len(resp.json()["ranked"]) == 1
