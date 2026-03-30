"""CineMatch ranker service — Stage-2 re-ranking microservice.

Accepts Stage-1 pgvector candidates from the Go backend and re-scores them
using either feature-linear-v1 (explicit weights) or lambdamart-v1 (learned
LightGBM model). The Go backend selects the model via the `model` field in
the RankRequest body.

Internal service only — not exposed to the browser. The Go backend calls
POST /rank after match_movies returns Stage-1 candidates.
"""

import logging
import os
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse

from models import RankRequest, RankResponse
import ranker as linear_ranker
import lambdamart_ranker

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s %(message)s")
logger = logging.getLogger("cinematch.ranker")


@asynccontextmanager
async def lifespan(app: FastAPI):  # noqa: ARG001
    logger.info("ranker service starting")
    model_path = os.environ.get("LAMBDAMART_MODEL_PATH")
    try:
        lambdamart_ranker.load_model(model_path)
        logger.info("lambdamart-v1 model loaded")
    except Exception:
        logger.warning("lambdamart-v1 model not available, feature-linear-v1 only")
    yield
    logger.info("ranker service shutting down")


app = FastAPI(
    title="CineMatch Ranker",
    description="Stage-2 re-ranking service for the two-stage recommendation pipeline.",
    version="1.0.0",
    lifespan=lifespan,
    # Disable the default /docs and /redoc in production — internal service only.
    docs_url="/docs",
    redoc_url=None,
)


@app.post("/rank", response_model=RankResponse, summary="Re-rank Stage-1 candidates")
def rank_candidates(request: RankRequest) -> RankResponse:
    """Re-score and sort Stage-1 candidates using user preference features.

    Called by the Go backend after the match_movies pgvector RPC returns
    the top-50 cosine-similarity candidates. Returns the top-N re-ranked results.

    Request body:
        candidates: list of MovieCandidate from the Go backend
        user_features: optional genre preferences and vote threshold
        top_n: number of results to return (default 20, max 50)

    Response:
        ranked: list of {movie_id, score, rank} sorted by descending score
        model_version: identifier for the active ranker (for eval tracking)
    """
    logger.info(
        "ranking request",
        extra={
            "candidate_count": len(request.candidates),
            "top_n": request.top_n,
            "has_genre_prefs": bool(request.user_features.preferred_genres),
        },
    )
    if request.model == "lambdamart-v1":
        return lambdamart_ranker.rank(request)
    return linear_ranker.rank(request)


@app.get("/health", summary="Health check")
def health() -> JSONResponse:
    """Returns 200 if the ranker service is running."""
    return JSONResponse({"status": "ok", "service": "cinematch-ranker"})


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8000, log_level="info")
