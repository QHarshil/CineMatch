"""Feature-weighted linear ranker for CineMatch Stage-2 re-ranking.

This replaces the Go backend's stub passthrough with a principled scoring
function that combines pgvector cosine similarity with movie quality signals
and user preference features.

Architecture note:
    This is an explicit feature model, not a learned one — we don't have
    enough interaction data yet to train LambdaMART. Once Task 11 (eval
    pipeline) produces sufficient labelled data, swap rank() for a
    lightgbm.LGBMRanker fitted on (query, candidate, label) triples.
    The interface (RankRequest → RankResponse) stays the same.

Scoring formula:
    score = 0.50 * similarity          # dominant: pgvector cosine sim
          + 0.25 * quality             # vote_average normalised to [0, 1]
          + 0.15 * log_popularity      # log-scaled popularity (prevents blockbusters drowning everything)
          + 0.10 * genre_overlap       # fraction of candidate genres matching user preference

    An optional vote-floor penalty halves the score when vote_average falls
    below the user's min_vote_preference threshold.
"""

import math

from models import CandidateMovie, RankRequest, RankedMovie, RankResponse

MODEL_VERSION = "feature-linear-v1"

# Popularity is unbounded (The Dark Knight ~2200, indie films <10).
# Log-normalising against this ceiling keeps blockbusters from dominating.
_POPULARITY_LOG_CEIL = math.log1p(3000.0)

# vote_average lives in [0, 10]; we rescale to [0, 1].
_VOTE_SCALE = 10.0

_W_SIMILARITY = 0.50
_W_QUALITY = 0.25
_W_POPULARITY = 0.15
_W_GENRE = 0.10


def _log_popularity_score(popularity: float) -> float:
    """Map raw popularity to [0, 1] via log scale."""
    return min(math.log1p(max(popularity, 0.0)) / _POPULARITY_LOG_CEIL, 1.0)


def _genre_overlap(candidate_genres: list[str], preferred_genres: list[str]) -> float:
    """Fraction of candidate genres present in user's preferred set.

    Returns 0.5 when the user has no recorded preferences — neutral, not penalising.
    """
    if not preferred_genres:
        return 0.5
    if not candidate_genres:
        return 0.0
    preferred_set = {g.lower() for g in preferred_genres}
    matches = sum(1 for g in candidate_genres if g.lower() in preferred_set)
    return matches / len(candidate_genres)


def _score_candidate(
    candidate: CandidateMovie, preferred_genres: list[str], min_vote: float
) -> float:
    """Compute composite ranking score for a single candidate."""
    quality = candidate.vote_average / _VOTE_SCALE
    log_pop = _log_popularity_score(candidate.popularity)
    genre = _genre_overlap(candidate.genres, preferred_genres)

    score = (
        _W_SIMILARITY * candidate.similarity
        + _W_QUALITY * quality
        + _W_POPULARITY * log_pop
        + _W_GENRE * genre
    )

    # Penalise movies that fall below the user's stated vote threshold.
    # A 50% penalty rather than a hard cutoff avoids eliminating borderline
    # movies that score very highly on similarity.
    if min_vote > 0.0 and candidate.vote_average < min_vote:
        score *= 0.5

    return score


def rank(request: RankRequest) -> RankResponse:
    """Re-rank Stage-1 candidates and return the top-N by composite score.

    Args:
        request: validated RankRequest containing candidates and user features.

    Returns:
        RankResponse with candidates sorted by descending score, capped at top_n.
    """
    preferred = request.user_features.preferred_genres
    min_vote = request.user_features.min_vote_preference

    scored = [
        (candidate.movie_id, _score_candidate(candidate, preferred, min_vote))
        for candidate in request.candidates
    ]
    scored.sort(key=lambda x: x[1], reverse=True)

    top = scored[: request.top_n]
    ranked = [
        RankedMovie(movie_id=movie_id, score=round(score, 6), rank=i + 1)
        for i, (movie_id, score) in enumerate(top)
    ]

    return RankResponse(ranked=ranked, model_version=MODEL_VERSION)
