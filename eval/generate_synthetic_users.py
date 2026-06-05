"""Generate synthetic user interaction data for ranker training and evaluation.

Creates 200 users across 8 taste profiles, each with 20-80 interactions against
the real movie catalog. A user's interactions are driven by a "true utility"
that mixes a linear genre signal with three deliberately non-linear effects:

  - era preference: each profile favours a release decade (a peaked bump)
  - non-monotonic quality: most profiles have a vote-average sweet spot rather
    than "higher is always better"
  - recency-by-genre: a recency bias that only applies inside loved genres

The genre signal is stored separately as `affinity_score`, which is what the
Stage-2 linear ranker sees as `similarity`. The non-linear effects are NOT in
`affinity_score`, so the fixed-weight linear ranker (similarity, quality,
popularity, genre overlap) structurally cannot represent them, while a tree
model with decade / is_recent / vote_average features can. This is what lets
LambdaMART beat the linear baseline on held-out data instead of merely matching
a formula it was generated from.

Output: eval/data/synthetic_interactions.parquet, eval/data/movies.parquet
"""

import os
import uuid
from pathlib import Path

import numpy as np
import pandas as pd

SEED = 42
NUM_USERS = 200
MIN_INTERACTIONS = 20
MAX_INTERACTIONS = 80
DATA_DIR = Path(__file__).resolve().parent / "data"

# Decades are ordinals: 1970s=0, 1980s=1, ... 2020s=5 (pre-1970 clamps to 0),
# matching build_training_data.py. preferred_decade is where a profile peaks.
# quality_style: "sweet_spot" (peaks mid, non-monotonic), "acclaimed" (monotonic
# up), "contrarian" (prefers lower-rated hidden gems). recency_pref biases toward
# (>0) or against (<0) post-2021 releases, applied only within loved genres.
TASTE_PROFILES: dict[str, dict] = {
    "action_fan": {
        "loved_genres": ["Action", "Adventure", "Science Fiction"],
        "liked_genres": ["Thriller", "Fantasy"],
        "disliked_genres": ["Romance", "Drama", "Documentary"],
        "preferred_decade": 5, "quality_style": "acclaimed", "recency_pref": 0.8,
        "weight": 0.15,
    },
    "arthouse": {
        "loved_genres": ["Drama", "History", "Documentary"],
        "liked_genres": ["Mystery", "War", "Music"],
        "disliked_genres": ["Action", "Animation", "Family"],
        "preferred_decade": 2, "quality_style": "sweet_spot", "recency_pref": -0.9,
        "weight": 0.12,
    },
    "comedy_lover": {
        "loved_genres": ["Comedy", "Animation", "Family"],
        "liked_genres": ["Romance", "Adventure"],
        "disliked_genres": ["Horror", "War", "Documentary"],
        "preferred_decade": 4, "quality_style": "sweet_spot", "recency_pref": 0.3,
        "weight": 0.14,
    },
    "horror_buff": {
        "loved_genres": ["Horror", "Thriller", "Mystery"],
        "liked_genres": ["Crime", "Science Fiction"],
        "disliked_genres": ["Comedy", "Family", "Animation", "Romance"],
        "preferred_decade": 3, "quality_style": "contrarian", "recency_pref": 0.2,
        "weight": 0.10,
    },
    "scifi_nerd": {
        "loved_genres": ["Science Fiction", "Fantasy", "Adventure"],
        "liked_genres": ["Action", "Animation", "Mystery"],
        "disliked_genres": ["Romance", "History", "Documentary", "Western"],
        "preferred_decade": 5, "quality_style": "acclaimed", "recency_pref": 0.6,
        "weight": 0.12,
    },
    "drama_enthusiast": {
        "loved_genres": ["Drama", "Romance", "Crime"],
        "liked_genres": ["Mystery", "Thriller", "History"],
        "disliked_genres": ["Horror", "Animation", "Science Fiction"],
        "preferred_decade": 2, "quality_style": "sweet_spot", "recency_pref": -0.6,
        "weight": 0.13,
    },
    "thriller_junkie": {
        "loved_genres": ["Thriller", "Crime", "Mystery"],
        "liked_genres": ["Action", "Horror", "Drama"],
        "disliked_genres": ["Comedy", "Family", "Animation", "Romance"],
        "preferred_decade": 4, "quality_style": "sweet_spot", "recency_pref": 0.1,
        "weight": 0.10,
    },
    "generalist": {
        "loved_genres": [],
        "liked_genres": ["Action", "Comedy", "Drama", "Thriller", "Adventure"],
        "disliked_genres": [],
        "preferred_decade": 4, "quality_style": "acclaimed", "recency_pref": 0.0,
        "weight": 0.14,
    },
}

# Relative weights of each utility component. The non-linear terms (era, quality,
# recency) together outweigh the linear genre term, so a model that can only use
# the genre signal leaves most of the ranking signal on the table.
W_GENRE = 0.70
W_ERA = 1.00
W_QUALITY = 0.70
W_RECENCY = 0.60


def _genre_affinity(movie_genres, profile: dict) -> float:
    """Linear genre match in roughly [-1, 1]. Stored as affinity_score."""
    if movie_genres is None or len(movie_genres) == 0:
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


def _decade_ordinal(release_year: float) -> int:
    """Match build_training_data: 1970s=0, clamped at 0 for older films."""
    if not np.isfinite(release_year):
        return 3
    return max(0, int((release_year - 1970) // 10))


def _era_affinity(release_year: float, preferred_decade: int) -> float:
    """Peaked preference for a release decade. A Gaussian bump (width ~1.2
    decades) returns ~1 at the favourite decade and decays for others. The
    linear ranker has no era feature, so it cannot represent this at all."""
    d = _decade_ordinal(release_year)
    return float(np.exp(-((d - preferred_decade) ** 2) / (2 * 1.2 ** 2)))


def _quality_term(vote_average: float, style: str) -> float:
    """Quality preference in roughly [-1, 1].

    "acclaimed" is monotonic (the linear ranker's quality weight can match it);
    "sweet_spot" peaks around 7.0 and falls off for both bad and over-hyped
    films; "contrarian" rewards lower-rated hidden gems. The latter two are
    non-monotonic, which a single linear quality weight cannot express.
    """
    if style == "acclaimed":
        return (vote_average - 6.5) / 2.0
    if style == "contrarian":
        return (6.8 - vote_average) / 2.0
    # sweet_spot: downward parabola peaking at 7.2
    return 1.0 - ((vote_average - 7.2) / 1.5) ** 2


def _recency_term(release_year: float, genre_affinity: float, recency_pref: float) -> float:
    """Recency bias applied only inside loved genres (genre_affinity > 0).

    Encodes interactions like "I want recent sci-fi but classic drama" that
    depend jointly on release year and genre, which a per-feature linear model
    cannot capture."""
    if genre_affinity <= 0:
        return 0.0
    is_recent = 1.0 if (np.isfinite(release_year) and release_year >= 2021) else -1.0
    return recency_pref * is_recent


def _true_utility(genre_aff: float, movie_row: pd.Series, profile: dict) -> float:
    """Combined preference that drives which movies a user likes/watches/skips."""
    era = _era_affinity(movie_row["release_year"], profile["preferred_decade"])
    quality = _quality_term(movie_row["vote_average"], profile["quality_style"])
    recency = _recency_term(movie_row["release_year"], genre_aff, profile["recency_pref"])
    return W_GENRE * genre_aff + W_ERA * (era - 0.4) + W_QUALITY * quality + W_RECENCY * recency


def _interaction_type(utility: float, rng: np.random.Generator) -> str:
    """Map utility (plus noise) to an interaction type."""
    noisy = utility + rng.normal(0, 0.3)
    if noisy > 0.5:
        return rng.choice(["like", "watch"], p=[0.6, 0.4])
    elif noisy > 0.05:
        return rng.choice(["watch", "like", "skip"], p=[0.5, 0.2, 0.3])
    elif noisy > -0.35:
        return rng.choice(["skip", "watch", "dislike"], p=[0.5, 0.3, 0.2])
    return rng.choice(["dislike", "skip"], p=[0.6, 0.4])


def load_movies() -> pd.DataFrame:
    """Load the catalog. Prefers the cached parquet so the pipeline is
    reproducible offline; falls back to Supabase if the cache is absent."""
    cached = DATA_DIR / "movies.parquet"
    if cached.exists():
        df = pd.read_parquet(cached)
    else:
        df = _fetch_movies_from_supabase()
    df["release_year"] = pd.to_numeric(df["release_year"], errors="coerce")
    df["vote_average"] = pd.to_numeric(df["vote_average"], errors="coerce").fillna(6.5)
    df["popularity"] = pd.to_numeric(df["popularity"], errors="coerce").fillna(0.0)
    df["runtime"] = pd.to_numeric(df["runtime"], errors="coerce")
    return df


def _fetch_movies_from_supabase() -> pd.DataFrame:
    """Fetch all movies from Supabase (used only when the cache is missing)."""
    from supabase import create_client

    url = os.environ.get("SUPABASE_URL", "")
    key = os.environ.get("SUPABASE_SECRET_KEY", "")
    if not url or not key:
        env_path = Path(__file__).resolve().parent.parent / ".env"
        for line in env_path.read_text().splitlines():
            if "=" in line and not line.startswith("#"):
                k, v = (s.strip() for s in line.split("=", 1))
                if k == "SUPABASE_URL":
                    url = v
                elif k == "SUPABASE_SECRET_KEY":
                    key = v

    client = create_client(url, key)
    rows, offset, batch = [], 0, 1000
    while True:
        resp = client.table("movies").select(
            "id,tmdb_id,title,genres,release_year,vote_average,popularity,runtime"
        ).range(offset, offset + batch - 1).execute()
        rows.extend(resp.data)
        if len(resp.data) < batch:
            break
        offset += batch
    return pd.DataFrame(rows)


def generate_interactions(movies_df: pd.DataFrame) -> pd.DataFrame:
    """Generate synthetic interactions for NUM_USERS users."""
    rng = np.random.default_rng(SEED)

    profile_names = list(TASTE_PROFILES.keys())
    raw_weights = np.array([TASTE_PROFILES[p]["weight"] for p in profile_names])
    profile_weights = raw_weights / raw_weights.sum()
    user_profiles = rng.choice(profile_names, size=NUM_USERS, p=profile_weights)

    genres_list = movies_df["genres"].tolist()
    interactions = []

    for user_idx in range(NUM_USERS):
        user_id = str(uuid.UUID(bytes=bytes(rng.integers(0, 256, size=16, dtype=np.uint8))))
        profile = TASTE_PROFILES[user_profiles[user_idx]]
        num_interactions = int(rng.integers(MIN_INTERACTIONS, MAX_INTERACTIONS + 1))

        genre_aff = np.array([
            _genre_affinity(g if isinstance(g, (list, np.ndarray)) else [], profile)
            for g in genres_list
        ])
        utility = np.array([
            _true_utility(genre_aff[i], movies_df.iloc[i], profile)
            for i in range(len(movies_df))
        ])

        # Sample movies the user feels something about: probability rises with
        # utility but keeps a floor so disliked films still appear (and get
        # labelled as skips/dislikes), producing a realistic label mix.
        shifted = utility - utility.min() + 0.15
        probs = shifted / shifted.sum()
        chosen = rng.choice(len(movies_df), size=num_interactions, p=probs, replace=True)

        seen: set = set()
        for idx in chosen:
            row = movies_df.iloc[idx]
            if row["id"] in seen:
                continue
            seen.add(row["id"])
            interactions.append({
                "user_id": user_id,
                "movie_id": row["id"],
                "type": _interaction_type(utility[idx], rng),
                "profile": user_profiles[user_idx],
                # Genre signal only -> this is the Stage-2 `similarity` input.
                "affinity_score": round(float(genre_aff[idx]), 4),
                "movie_title": row["title"],
                "movie_genres": row["genres"],
                "vote_average": row["vote_average"],
                "popularity": row["popularity"],
                "release_year": row["release_year"],
                "runtime": row["runtime"],
            })

    return pd.DataFrame(interactions)


def main():
    print("Loading catalog...")
    movies_df = load_movies()
    print(f"  {len(movies_df)} movies")

    print("Generating synthetic interactions...")
    interactions_df = generate_interactions(movies_df)
    print(f"  {len(interactions_df)} interactions for {interactions_df['user_id'].nunique()} users")
    print("\nInteraction type distribution:")
    print(interactions_df["type"].value_counts().to_string())

    DATA_DIR.mkdir(exist_ok=True)
    out_path = DATA_DIR / "synthetic_interactions.parquet"
    interactions_df.to_parquet(out_path, index=False)
    print(f"\nSaved {out_path}")
    movies_df.to_parquet(DATA_DIR / "movies.parquet", index=False)
    print(f"Saved {DATA_DIR / 'movies.parquet'}")


if __name__ == "__main__":
    main()
