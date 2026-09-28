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
  /** "movie" or "tv". Absent on responses from a backend deploy predating TV support. */
  media_type?: "movie" | "tv";
}

/** One GET /discover result. A null rank means that retriever did not return the title. */
export interface SearchHit extends Movie {
  original_language?: string;
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

/** GET /recommend response from the Go backend. */
export interface RecommendResponse {
  movies: Movie[];
  source: "personalized" | "popular" | "similarity_fallback";
  model_version?: string;
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
