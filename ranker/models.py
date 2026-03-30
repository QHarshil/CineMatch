"""Pydantic models for the /rank endpoint request and response."""

from typing import Annotated
from pydantic import BaseModel, Field


class CandidateMovie(BaseModel):
    """A single Stage-1 retrieval result from the Go backend's match_movies RPC."""

    movie_id: str
    title: str
    genres: list[str]
    release_year: int
    vote_average: float
    popularity: float
    runtime: int
    similarity: Annotated[float, Field(ge=0.0, le=1.0)]


class UserFeatures(BaseModel):
    """Per-request user signals used to personalise Stage-2 re-ranking.

    All fields are optional so the ranker degrades gracefully when a user
    has few or no interactions recorded yet.
    """

    preferred_genres: list[str] = Field(
        default_factory=list,
        description="Genres the user has historically liked or watched.",
    )
    # Normalised vote threshold derived from the user's past ratings.
    # When present, movies below this threshold are penalised.
    min_vote_preference: float = Field(
        default=0.0,
        ge=0.0,
        le=10.0,
        description="Minimum vote_average the user tends to enjoy.",
    )


class RankRequest(BaseModel):
    """POST /rank request body."""

    candidates: Annotated[
        list[CandidateMovie],
        Field(min_length=1, description="Stage-1 candidates from pgvector kNN."),
    ]
    user_features: UserFeatures = Field(default_factory=UserFeatures)
    top_n: Annotated[int, Field(ge=1, le=50)] = 20
    model: str = Field(
        default="lambdamart-v1",
        description="Which ranker model to use: 'feature-linear-v1' or 'lambdamart-v1'.",
    )


class RankedMovie(BaseModel):
    """A single re-ranked result returned to the Go backend."""

    movie_id: str
    score: float
    rank: int


class RankResponse(BaseModel):
    """POST /rank response body."""

    ranked: list[RankedMovie]
    # Included so the Go backend can log which ranker version produced this result.
    # Useful when comparing ranker versions in offline eval (Task 11).
    model_version: str
