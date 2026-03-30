"""Generate synthetic user interaction data for LambdaMART training.

Creates 200 users across 8 taste profiles, each with 20-80 interactions
against the real movie catalog in Supabase. Interactions are weighted by
genre affinity with Gaussian noise so the data is realistic but not
deterministic.

Output: eval/data/synthetic_interactions.parquet
"""

import hashlib
import math
import os
import random
import uuid
from pathlib import Path

import numpy as np
import pandas as pd
from supabase import create_client

SEED = 42
NUM_USERS = 200
MIN_INTERACTIONS = 20
MAX_INTERACTIONS = 80

TASTE_PROFILES: dict[str, dict] = {
    "action_fan": {
        "loved_genres": ["Action", "Adventure", "Science Fiction"],
        "liked_genres": ["Thriller", "Fantasy"],
        "disliked_genres": ["Romance", "Drama", "Documentary"],
        "weight": 0.15,
    },
    "arthouse": {
        "loved_genres": ["Drama", "History", "Documentary"],
        "liked_genres": ["Mystery", "War", "Music"],
        "disliked_genres": ["Action", "Animation", "Family"],
        "weight": 0.12,
    },
    "comedy_lover": {
        "loved_genres": ["Comedy", "Animation", "Family"],
        "liked_genres": ["Romance", "Adventure"],
        "disliked_genres": ["Horror", "War", "Documentary"],
        "weight": 0.14,
    },
    "horror_buff": {
        "loved_genres": ["Horror", "Thriller", "Mystery"],
        "liked_genres": ["Crime", "Science Fiction"],
        "disliked_genres": ["Comedy", "Family", "Animation", "Romance"],
        "weight": 0.10,
    },
    "scifi_nerd": {
        "loved_genres": ["Science Fiction", "Fantasy", "Adventure"],
        "liked_genres": ["Action", "Animation", "Mystery"],
        "disliked_genres": ["Romance", "History", "Documentary", "Western"],
        "weight": 0.12,
    },
    "drama_enthusiast": {
        "loved_genres": ["Drama", "Romance", "Crime"],
        "liked_genres": ["Mystery", "Thriller", "History"],
        "disliked_genres": ["Horror", "Animation", "Science Fiction"],
        "weight": 0.13,
    },
    "thriller_junkie": {
        "loved_genres": ["Thriller", "Crime", "Mystery"],
        "liked_genres": ["Action", "Horror", "Drama"],
        "disliked_genres": ["Comedy", "Family", "Animation", "Romance"],
        "weight": 0.10,
    },
    "generalist": {
        "loved_genres": [],
        "liked_genres": ["Action", "Comedy", "Drama", "Thriller", "Adventure"],
        "disliked_genres": [],
        "weight": 0.14,
    },
}


def _genre_affinity(movie_genres: list[str], profile: dict) -> float:
    """Score how well a movie's genres match a taste profile.

    Returns a value roughly in [-1, 1] indicating affinity.
    """
    if not movie_genres:
        return 0.0

    loved = set(profile["loved_genres"])
    liked = set(profile["liked_genres"])
    disliked = set(profile["disliked_genres"])

    score = 0.0
    for g in movie_genres:
        if g in loved:
            score += 1.0
        elif g in liked:
            score += 0.4
        elif g in disliked:
            score -= 0.7
    return score / len(movie_genres)


def _quality_boost(vote_average: float) -> float:
    """Higher-rated movies are more likely to get positive interactions."""
    return (vote_average - 5.0) / 10.0  # maps [0,10] to [-0.5, 0.5]


def _interaction_type(affinity_score: float, rng: np.random.Generator) -> str:
    """Determine interaction type from affinity score + noise.

    Higher affinity -> more likely to get like/watch.
    Lower affinity -> more likely to get dislike/skip.
    """
    noisy = affinity_score + rng.normal(0, 0.3)

    if noisy > 0.5:
        return rng.choice(["like", "watch"], p=[0.6, 0.4])
    elif noisy > 0.0:
        return rng.choice(["watch", "like", "skip"], p=[0.5, 0.2, 0.3])
    elif noisy > -0.3:
        return rng.choice(["skip", "watch", "dislike"], p=[0.5, 0.3, 0.2])
    else:
        return rng.choice(["dislike", "skip"], p=[0.6, 0.4])


def fetch_movies() -> pd.DataFrame:
    """Fetch all movies from Supabase."""
    url = os.environ.get("SUPABASE_URL", "")
    key = os.environ.get("SUPABASE_SECRET_KEY", "")

    if not url or not key:
        env_path = Path(__file__).resolve().parent.parent / ".env"
        if env_path.exists():
            for line in env_path.read_text().splitlines():
                if "=" in line and not line.startswith("#"):
                    k, v = line.split("=", 1)
                    k, v = k.strip(), v.strip()
                    if k == "SUPABASE_URL":
                        url = v
                    elif k == "SUPABASE_SECRET_KEY":
                        key = v

    client = create_client(url, key)
    rows = []
    offset = 0
    batch_size = 1000
    while True:
        resp = client.table("movies").select(
            "id,tmdb_id,title,genres,release_year,vote_average,popularity,runtime"
        ).range(offset, offset + batch_size - 1).execute()
        rows.extend(resp.data)
        if len(resp.data) < batch_size:
            break
        offset += batch_size

    return pd.DataFrame(rows)


def generate_interactions(movies_df: pd.DataFrame) -> pd.DataFrame:
    """Generate synthetic interactions for 200 users."""
    rng = np.random.default_rng(SEED)
    random.seed(SEED)

    # Assign users to profiles based on weights
    profile_names = list(TASTE_PROFILES.keys())
    profile_weights = [TASTE_PROFILES[p]["weight"] for p in profile_names]
    # Normalize weights
    total = sum(profile_weights)
    profile_weights = [w / total for w in profile_weights]

    user_profiles = rng.choice(profile_names, size=NUM_USERS, p=profile_weights)

    interactions = []

    for user_idx in range(NUM_USERS):
        user_id = str(uuid.UUID(bytes=bytes(rng.integers(0, 256, size=16, dtype=np.uint8))))
        profile_name = user_profiles[user_idx]
        profile = TASTE_PROFILES[profile_name]

        num_interactions = rng.integers(MIN_INTERACTIONS, MAX_INTERACTIONS + 1)

        # Score all movies by affinity, then sample with probability proportional
        # to absolute affinity (users interact more with movies they feel strongly about)
        affinities = movies_df.apply(
            lambda row: _genre_affinity(row["genres"], profile)
            + _quality_boost(row["vote_average"]),
            axis=1,
        ).values

        # Convert to sampling weights: shift so all positive, add floor
        shifted = affinities - affinities.min() + 0.1
        probs = shifted / shifted.sum()

        # Sample movie indices (with replacement to allow re-interactions)
        chosen_indices = rng.choice(
            len(movies_df), size=int(num_interactions), p=probs, replace=True
        )
        # Deduplicate — one interaction per movie per user
        seen_movies = set()
        for idx in chosen_indices:
            movie_row = movies_df.iloc[idx]
            movie_id = movie_row["id"]
            if movie_id in seen_movies:
                continue
            seen_movies.add(movie_id)

            affinity = (
                _genre_affinity(movie_row["genres"], profile)
                + _quality_boost(movie_row["vote_average"])
            )
            itype = _interaction_type(affinity, rng)

            interactions.append({
                "user_id": user_id,
                "movie_id": movie_id,
                "type": itype,
                "profile": profile_name,
                "affinity_score": round(affinity, 4),
                "movie_title": movie_row["title"],
                "movie_genres": movie_row["genres"],
                "vote_average": movie_row["vote_average"],
                "popularity": movie_row["popularity"],
                "release_year": movie_row["release_year"],
                "runtime": movie_row["runtime"],
            })

    return pd.DataFrame(interactions)


def main():
    print("Fetching movies from Supabase...")
    movies_df = fetch_movies()
    print(f"  {len(movies_df)} movies loaded")

    print("Generating synthetic interactions...")
    interactions_df = generate_interactions(movies_df)
    print(f"  {len(interactions_df)} interactions for {interactions_df['user_id'].nunique()} users")

    # Distribution summary
    print("\nInteraction type distribution:")
    print(interactions_df["type"].value_counts().to_string())
    print("\nProfile distribution:")
    print(interactions_df["profile"].value_counts().to_string())
    print(f"\nInteractions per user: min={interactions_df.groupby('user_id').size().min()}, "
          f"max={interactions_df.groupby('user_id').size().max()}, "
          f"mean={interactions_df.groupby('user_id').size().mean():.1f}")

    # Save
    out_dir = Path(__file__).resolve().parent / "data"
    out_dir.mkdir(exist_ok=True)
    out_path = out_dir / "synthetic_interactions.parquet"
    interactions_df.to_parquet(out_path, index=False)
    print(f"\nSaved to {out_path}")

    # Also save movies for training pipeline
    movies_path = out_dir / "movies.parquet"
    movies_df.to_parquet(movies_path, index=False)
    print(f"Saved movies to {movies_path}")


if __name__ == "__main__":
    main()
