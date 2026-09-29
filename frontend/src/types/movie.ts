/** Movie as returned by the Go backend (embedding excluded). */
export interface Movie {
  id: string;
  tmdb_id: number;
  title: string;
  overview: string;
  genres: string[];
  release_year: number;
  poster_path: string;
  backdrop_path?: string;
  vote_average: number;
  popularity: number;
  runtime: number;
  /** "movie" or "tv". */
  media_type?: "movie" | "tv";
  /** ISO 639-1 code such as "en" or "ko". */
  original_language?: string;
}

/** A title returned by the match_movies kNN search, with its cosine similarity to the query. */
export interface SimilarTitle extends Movie {
  similarity: number;
}

/** GET /api/similar, the kNN demo on How It Works. */
export interface SimilarTitlesResponse {
  seed: string | null;
  neighbors: Pick<SimilarTitle, "id" | "title" | "genres" | "vote_average" | "poster_path" | "similarity">[];
}

/** One GET /discover result. A null rank means that retriever did not return the title. */
export interface SearchHit extends Movie {
  /** Cosine similarity between the query and the title's embedding. */
  similarity: number | null;
  semantic_rank: number | null;
  keyword_rank: number | null;
  title_rank: number | null;
  /** Reciprocal rank fusion score. */
  score: number;
}

/** How /discover matched: embeddings + keywords + titles, keywords only, or the offline cache. */
export type RetrievalMode = "hybrid" | "keyword" | "cached";

export interface DiscoverResponse {
  results: SearchHit[];
  retrieval: RetrievalMode;
}

/** A ranker feature that raised a pick's score, with its SHAP contribution. */
export interface RankingFactor {
  feature: "similarity" | "vote_average" | "log_popularity" | "decade" | "is_recent";
  contribution: number;
}

/** Why a title was recommended. */
export interface RecommendationExplanation {
  /** Cosine similarity to the taste vector. */
  similarity: number;
  because_you_liked?: { id: string; title: string; similarity: number };
  factors?: RankingFactor[];
}

/** GET /recommend response from the Go backend. */
export interface RecommendResponse {
  movies: Movie[];
  source: "personalized" | "popular" | "similarity_fallback";
  model_version?: string;
  /** Keyed by movie ID; absent for popular feeds. */
  explanations?: Record<string, RecommendationExplanation>;
}

/** Interaction types the user can record. */
export type InteractionType = "like" | "dislike" | "watch" | "skip";

/** Response from GET /interactions?movie_id=UUID */
export interface InteractionState {
  interactions: InteractionType[];
  rating: number | null;
}

/** Response from POST /interactions (toggle) */
export interface ToggleResponse {
  action: "added" | "removed";
  type: string;
}
