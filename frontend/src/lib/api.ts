import type { Movie, RecommendResponse, InteractionType, InteractionState, ToggleResponse } from "@/types/movie";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** Thrown when the API returns 429 Too Many Requests. */
export class RateLimitError extends Error {
  constructor() {
    super("Too many requests");
    this.name = "RateLimitError";
  }
}

async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (res.status === 429) {
    throw new RateLimitError();
  }

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }

  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

/** Attach the Supabase JWT to authenticated requests.
 *  Sends via both Authorization and X-Authorization because Railway's
 *  CDN edge (Fastly) strips the standard Authorization header. */
function authHeaders(token: string): HeadersInit {
  return {
    Authorization: `Bearer ${token}`,
    "X-Authorization": `Bearer ${token}`,
  };
}

// ---------------------------------------------------------------------------
// Public endpoints
// ---------------------------------------------------------------------------

export function fetchMovies(limit = 20, offset = 0): Promise<Movie[]> {
  return apiFetch<Movie[]>(`/movies?limit=${limit}&offset=${offset}`);
}

export function fetchMovieById(id: string): Promise<Movie> {
  return apiFetch<Movie>(`/movies/${id}`);
}

export function searchMovies(query: string, limit = 20): Promise<Movie[]> {
  return apiFetch<Movie[]>(
    `/search?q=${encodeURIComponent(query)}&limit=${limit}`
  );
}

// ---------------------------------------------------------------------------
// Authenticated endpoints
// ---------------------------------------------------------------------------

export function fetchRecommendations(
  token: string
): Promise<RecommendResponse> {
  return apiFetch<RecommendResponse>("/recommend", {
    headers: authHeaders(token),
  });
}

export function toggleInteraction(
  token: string,
  movieId: string,
  type: InteractionType
): Promise<ToggleResponse> {
  return apiFetch<ToggleResponse>("/interactions", {
    method: "POST",
    headers: authHeaders(token),
    body: JSON.stringify({ movie_id: movieId, type }),
  });
}

export function fetchInteractionState(
  token: string,
  movieId: string
): Promise<InteractionState> {
  return apiFetch<InteractionState>(
    `/interactions?movie_id=${movieId}`,
    { headers: authHeaders(token) }
  );
}

export function submitRating(
  token: string,
  movieId: string,
  score: number
): Promise<void> {
  return apiFetch("/ratings", {
    method: "PUT",
    headers: authHeaders(token),
    body: JSON.stringify({ movie_id: movieId, score }),
  });
}
