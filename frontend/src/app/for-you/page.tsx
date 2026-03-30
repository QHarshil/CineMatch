"use client";

import { useCallback, useRef, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { fetchRecommendations } from "@/lib/api";
import { ScrollRow } from "@/components/scroll-row";
import type { Movie } from "@/types/movie";
import Link from "next/link";
import Image from "next/image";
import { createSupabaseBrowserClient } from "@/lib/supabase-browser";

const TMDB_IMAGE = "https://image.tmdb.org/t/p/w342";

const DEMO_PROFILES = [
  {
    id: "scifi-thriller",
    label: "Sci-fi & Thriller",
    genres: ["Science Fiction", "Thriller"],
    description: "Inception, Interstellar, Blade Runner",
  },
  {
    id: "comedy-drama",
    label: "Comedy & Drama",
    genres: ["Comedy", "Drama"],
    description: "Parasite, Grand Budapest Hotel",
  },
  {
    id: "action-adventure",
    label: "Action & Adventure",
    genres: ["Action", "Adventure"],
    description: "Mad Max, John Wick, The Dark Knight",
  },
];

const MOVIE_FIELDS =
  "id,tmdb_id,title,overview,genres,release_year,poster_path,backdrop_path,vote_average,popularity,runtime";

interface BecauseYouLikedSection {
  likedMovie: Movie;
  similarMovies: Movie[];
}

export default function ForYouPage() {
  const { session, loading: authLoading } = useAuth();
  const [topPicks, setTopPicks] = useState<Movie[]>([]);
  const [popular, setPopular] = useState<Movie[]>([]);
  const [becauseYouLiked, setBecauseYouLiked] = useState<BecauseYouLikedSection[]>([]);
  const [source, setSource] = useState("");
  const [loading, setLoading] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [demoProfile, setDemoProfile] = useState<string | null>(null);
  const [backdropMovies, setBackdropMovies] = useState<Movie[]>([]);
  const supabase = useRef(createSupabaseBrowserClient());
  const fetchedRef = useRef(false);

  const fetchPopularMovies = useCallback(async () => {
    const { data } = await supabase.current
      .from("movies")
      .select(MOVIE_FIELDS)
      .order("popularity", { ascending: false })
      .limit(20);
    return (data ?? []) as Movie[];
  }, []);

  const fetchDemoRecommendations = useCallback(
    async (genres: string[]) => {
      const { data } = await supabase.current
        .from("movies")
        .select(MOVIE_FIELDS)
        .overlaps("genres", genres)
        .order("vote_average", { ascending: false })
        .limit(20);
      return (data ?? []) as Movie[];
    },
    []
  );

  /** Fetch the user's recent liked movies and find similar titles for each. */
  const fetchBecauseYouLiked = useCallback(async (userId: string): Promise<BecauseYouLikedSection[]> => {
    // Get the 3 most recent "like" interactions
    const { data: interactions } = await supabase.current
      .from("interactions")
      .select("movie_id")
      .eq("user_id", userId)
      .eq("type", "like")
      .order("created_at", { ascending: false })
      .limit(3);

    if (!interactions || interactions.length === 0) return [];

    const likedMovieIds = interactions.map((i) => i.movie_id as string);

    // Fetch the liked movies themselves
    const { data: likedMovies } = await supabase.current
      .from("movies")
      .select(MOVIE_FIELDS)
      .in("id", likedMovieIds);

    if (!likedMovies || likedMovies.length === 0) return [];

    // For each liked movie, find similar movies by genre overlap
    const sections: BecauseYouLikedSection[] = [];
    const seenMovieIds = new Set(likedMovieIds);

    for (const liked of likedMovies as Movie[]) {
      const topGenres = liked.genres.slice(0, 2);
      if (topGenres.length === 0) continue;

      const { data: similar } = await supabase.current
        .from("movies")
        .select(MOVIE_FIELDS)
        .neq("id", liked.id)
        .overlaps("genres", topGenres)
        .gte("vote_average", Math.max(0, liked.vote_average - 2))
        .order("popularity", { ascending: false })
        .limit(20);

      // Filter out movies already shown in other sections
      const filtered = ((similar ?? []) as Movie[]).filter((m) => !seenMovieIds.has(m.id));
      filtered.forEach((m) => seenMovieIds.add(m.id));

      if (filtered.length > 0) {
        sections.push({ likedMovie: liked, similarMovies: filtered });
      }
    }

    return sections;
  }, []);

  // Fetch authenticated user's recommendations
  const fetchAuthRecs = useCallback(async () => {
    if (!session || fetchedRef.current) return;
    fetchedRef.current = true;
    setLoading(true);
    try {
      const [recResult, popularResult, likedSections] = await Promise.all([
        fetchRecommendations(session.access_token).catch(() => null),
        fetchPopularMovies(),
        fetchBecauseYouLiked(session.user.id).catch(() => [] as BecauseYouLikedSection[]),
      ]);
      if (recResult) {
        setTopPicks(recResult.movies);
        setSource(recResult.source);
      }
      setPopular(popularResult);
      setBecauseYouLiked(likedSections);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
      setFetched(true);
    }
  }, [session, fetchPopularMovies, fetchBecauseYouLiked]);

  // Auto-fetch for authenticated users
  if (session && !fetched && !loading && !fetchedRef.current) {
    fetchAuthRecs();
  }

  // Fetch backdrop movies for unauthenticated state
  if (!session && !authLoading && backdropMovies.length === 0) {
    supabase.current
      .from("movies")
      .select("id,poster_path")
      .not("poster_path", "is", null)
      .order("popularity", { ascending: false })
      .limit(12)
      .then(({ data }) => {
        if (data) setBackdropMovies(data as Movie[]);
      });
  }

  async function handleDemoProfile(profile: typeof DEMO_PROFILES[number]) {
    setDemoProfile(profile.id);
    setLoading(true);
    setFetched(false);
    setBecauseYouLiked([]);
    try {
      const [demoResult, popularResult] = await Promise.all([
        fetchDemoRecommendations(profile.genres),
        fetchPopularMovies(),
      ]);
      setTopPicks(demoResult);
      setSource("demo");
      setPopular(popularResult);
    } catch {
      setError("Failed to load demo recommendations");
    } finally {
      setLoading(false);
      setFetched(true);
    }
  }

  const isAuthLoading = authLoading;
  const isDataLoading = loading;

  // ── Loading skeleton ──────────────────────────────────────────
  if (isAuthLoading || isDataLoading) {
    return (
      <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
        <div className="h-7 w-24 bg-surface animate-pulse mb-10" />
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="mb-12">
            <div className="h-5 w-48 bg-surface animate-pulse mb-4" />
            <div className="flex gap-4 overflow-hidden">
              {Array.from({ length: 6 }).map((_, j) => (
                <div key={j} className="w-[160px] shrink-0">
                  <div className="aspect-[2/3] bg-surface animate-pulse" />
                  <div className="h-4 w-3/4 bg-surface animate-pulse mt-2" />
                  <div className="h-3 w-1/3 bg-surface animate-pulse mt-1" />
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  // ── Unauthenticated: auth gate + demo profiles ────────────────
  if (!session && !demoProfile) {
    return (
      <div className="relative min-h-screen overflow-hidden">
        {/* Blurred poster grid background */}
        <div className="absolute inset-0 grid grid-cols-4 sm:grid-cols-6 gap-2 p-4 opacity-[0.08] blur-sm pointer-events-none">
          {backdropMovies.map((m) => (
            <div key={m.id} className="aspect-[2/3] relative">
              {m.poster_path && (
                <Image
                  src={`${TMDB_IMAGE}${m.poster_path}`}
                  alt=""
                  fill
                  sizes="120px"
                  className="object-cover"
                />
              )}
            </div>
          ))}
        </div>

        <div className="relative flex flex-col items-center justify-center pt-32 pb-20 px-4 text-center">
          <h1 className="font-heading text-3xl sm:text-4xl font-bold mb-3">
            Your personalized picks
          </h1>
          <p className="text-muted-foreground max-w-md mb-8">
            Sign in to get movie recommendations based on your taste,
            or try a demo profile below.
          </p>

          <Link
            href="/login"
            className="px-8 py-3 bg-gold text-background text-sm font-medium hover:bg-gold-dim transition-colors duration-200 mb-12"
          >
            Sign in
          </Link>

          {/* Demo profiles */}
          <div className="w-full max-w-lg">
            <p className="text-xs text-muted-foreground uppercase tracking-widest mb-4">
              Or try a demo profile
            </p>
            <div className="grid gap-3">
              {DEMO_PROFILES.map((profile) => (
                <button
                  key={profile.id}
                  onClick={() => handleDemoProfile(profile)}
                  className="flex items-center justify-between px-5 py-4 border border-border bg-surface/50 hover:border-gold/50 hover:bg-surface transition-colors duration-200 text-left"
                >
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {profile.label}
                    </p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {profile.description}
                    </p>
                  </div>
                  <span className="text-gold text-sm shrink-0 ml-4">&rarr;</span>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    );
  }

  // ── Error ─────────────────────────────────────────────────────
  if (error) {
    return (
      <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4">
        <p className="text-destructive text-sm">
          Failed to load recommendations.
        </p>
        <button
          onClick={() => window.location.reload()}
          className="px-5 py-2 border border-border text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Try again
        </button>
      </div>
    );
  }

  // ── Cold start (no personalized results) ──────────────────────
  const hasPersonalized = topPicks.length > 0 && source !== "popular";
  const isDemoMode = demoProfile !== null;

  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <h1 className="font-heading text-2xl font-bold mb-2">
        {isDemoMode ? "Demo Recommendations" : "For You"}
      </h1>

      {isDemoMode && (
        <p className="text-sm text-muted-foreground mb-8">
          Showing recommendations for the{" "}
          <span className="text-gold">
            {DEMO_PROFILES.find((p) => p.id === demoProfile)?.label}
          </span>{" "}
          profile.{" "}
          <Link href="/login" className="text-gold hover:text-gold-dim transition-colors">
            Sign in
          </Link>{" "}
          to get your own.
        </p>
      )}

      {/* Cold-start banner */}
      {!hasPersonalized && !isDemoMode && (
        <div className="mb-10 px-5 py-4 border border-border bg-surface/50">
          <p className="text-sm text-foreground">
            Rate some movies to unlock personalized recommendations.
          </p>
          <Link
            href="/browse"
            className="inline-block mt-2 text-sm text-gold hover:text-gold-dim transition-colors"
          >
            Browse movies &rarr;
          </Link>
        </div>
      )}

      {/* Top Picks section */}
      {topPicks.length > 0 && (
        <div className="mb-12">
          <ScrollRow
            title={isDemoMode ? "Top Picks" : "Top Picks for You"}
            movies={topPicks}
          />
        </div>
      )}

      {/* Because you liked X — personalized sections */}
      {becauseYouLiked.map((section) => (
        <div key={section.likedMovie.id} className="mb-12">
          <ScrollRow
            title={`Because you liked ${section.likedMovie.title}`}
            movies={section.similarMovies}
          />
        </div>
      ))}

      {/* Popular right now — always shown */}
      {popular.length > 0 && (
        <div className="mb-12">
          <ScrollRow
            title="Popular Right Now"
            movies={popular}
            seeAllHref="/browse"
          />
        </div>
      )}
    </div>
  );
}
