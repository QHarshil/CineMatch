"""Measure Stage-2 re-ranking latency.

Sends repeated POST /rank requests with a realistic 50-candidate payload (the
Stage-1 retrieval size) and reports p50/p95/p99 of the ranker's response time.
Run against a locally running ranker to isolate model-inference latency from
network round-trips:

    cd ranker && .venv/bin/uvicorn main:app --port 8000   # in one shell
    cd eval && .venv/bin/python benchmark_latency.py       # in another

Set RANKER_URL to benchmark a deployed instance instead.
"""

import os
import statistics
import time

import httpx

RANKER_URL = os.environ.get("RANKER_URL", "http://localhost:8000")
NUM_REQUESTS = int(os.environ.get("NUM_REQUESTS", "200"))
WARMUP = 10
CANDIDATE_COUNT = 50


def build_payload() -> dict:
    """A representative request: 50 retrieved candidates, top 20 returned."""
    candidates = [
        {
            "movie_id": f"aaaaaaaa-0000-0000-0000-{i:012d}",
            "title": f"Movie {i}",
            "genres": ["Drama", "Thriller"],
            "release_year": 2000 + (i % 25),
            "vote_average": 5.0 + (i % 50) / 10.0,
            "popularity": float(10 + i * 7),
            "runtime": 100 + (i % 60),
            "similarity": round(0.99 - i * 0.01, 4),
        }
        for i in range(CANDIDATE_COUNT)
    ]
    return {
        "candidates": candidates,
        "user_features": {"user_like_ratio": 0.6, "user_interaction_count": 40},
        "top_n": 20,
        "model": "lambdamart-v1",
    }


def main():
    payload = build_payload()
    with httpx.Client(timeout=10.0) as client:
        for _ in range(WARMUP):
            client.post(f"{RANKER_URL}/rank", json=payload).raise_for_status()

        latencies_ms = []
        for _ in range(NUM_REQUESTS):
            start = time.perf_counter()
            resp = client.post(f"{RANKER_URL}/rank", json=payload)
            resp.raise_for_status()
            latencies_ms.append((time.perf_counter() - start) * 1000.0)

    latencies_ms.sort()

    def pct(p: float) -> float:
        return latencies_ms[min(len(latencies_ms) - 1, int(p / 100.0 * len(latencies_ms)))]

    print(f"POST /rank  ({CANDIDATE_COUNT} candidates -> top 20, {NUM_REQUESTS} requests, model=lambdamart-v1)")
    print(f"  mean {statistics.mean(latencies_ms):.2f} ms")
    print(f"  p50  {pct(50):.2f} ms")
    print(f"  p95  {pct(95):.2f} ms")
    print(f"  p99  {pct(99):.2f} ms")
    print(f"  max  {latencies_ms[-1]:.2f} ms")


if __name__ == "__main__":
    main()
