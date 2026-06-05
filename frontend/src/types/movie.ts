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
