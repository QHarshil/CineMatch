"use client";

import { useCallback, useRef, useState } from "react";
import Image from "next/image";
import { Loader2, Search } from "lucide-react";

interface SeedMovie {
  id: string;
  title: string;
  poster_path: string;
}

interface Neighbor {
  id: string;
  title: string;
  genres: string[];
  vote_average: number;
  poster_path: string;
  similarity: number;
}

export function SimilarMoviesDemo({
  seedMovies,
}: {
  seedMovies: SeedMovie[];
}) {
  const [selectedId, setSelectedId] = useState("");
  const [neighbors, setNeighbors] = useState<Neighbor[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const handleSelect = useCallback((movieId: string) => {
    setSelectedId(movieId);
    if (!movieId) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setSearched(true);

    fetch(`/api/similar?movieId=${movieId}`, { signal: controller.signal })
      .then((res) => res.json())
      .then((data) => {
        if (!controller.signal.aborted) setNeighbors(data.neighbors ?? []);
      })
      .catch(() => {
        if (!controller.signal.aborted) setNeighbors([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
  }, []);

  const selected = seedMovies.find((m) => m.id === selectedId);

  return (
    <div className="border border-border bg-surface/50 p-6 sm:p-8">
      <div className="flex flex-col sm:flex-row sm:items-end gap-4 mb-8">
        <div className="flex-1">
          <label
            htmlFor="seed-movie"
            className="block text-sm text-muted-foreground mb-2"
          >
            Choose a movie to find its nearest neighbors
          </label>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <select
              id="seed-movie"
              value={selectedId}
              onChange={(e) => handleSelect(e.target.value)}
              className="w-full bg-background border border-border pl-10 pr-4 py-2.5 text-sm text-foreground appearance-none cursor-pointer focus:outline-none focus:border-gold transition-colors"
            >
              <option value="">Select a movie...</option>
              {seedMovies.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.title}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {loading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-5 h-5 text-gold animate-spin" />
          <span className="ml-3 text-sm text-muted-foreground">
            Searching embedding space...
          </span>
        </div>
      )}

      {!loading && searched && neighbors.length > 0 && (
        <div className="space-y-6">
          {/* Seed movie label */}
          {selected && (
            <div className="flex items-center gap-3 pb-4 border-b border-border/50">
              {selected.poster_path && (
                <Image
                  src={`https://image.tmdb.org/t/p/w92${selected.poster_path}`}
                  alt={selected.title}
                  width={36}
                  height={54}
                  className="object-cover bg-surface"
                />
              )}
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-widest">
                  Seed movie
                </p>
                <p className="font-heading text-lg font-semibold">
                  {selected.title}
                </p>
              </div>
            </div>
          )}

          {/* Neighbors */}
          <div className="grid gap-3">
            {neighbors.map((n, i) => (
              <div
                key={n.id}
                className="flex items-center gap-4 py-3 px-4 bg-background/50 border border-border/30 transition-all duration-300 ease-out"
                style={{
                  animationDelay: `${i * 80}ms`,
                  animation: "fadeSlideIn 0.4s ease-out both",
                }}
              >
                <span className="text-xs text-muted-foreground font-mono w-5 shrink-0">
                  #{i + 1}
                </span>
                {n.poster_path && (
                  <Image
                    src={`https://image.tmdb.org/t/p/w92${n.poster_path}`}
                    alt={n.title}
                    width={32}
                    height={48}
                    className="object-cover bg-surface shrink-0"
                  />
                )}
                <div className="flex-1 min-w-0">
                  <p className="font-heading text-base font-medium truncate">
                    {n.title}
                  </p>
                  <p className="text-xs text-muted-foreground truncate">
                    {n.genres?.slice(0, 3).join(", ")}
                  </p>
                </div>
                <div className="text-right shrink-0">
                  <p className="text-sm font-mono text-gold">
                    {(n.similarity * 100).toFixed(1)}%
                  </p>
                  <p className="text-[10px] text-muted-foreground">
                    similarity
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {!loading && !searched && (
        <div className="text-center py-12">
          <p className="text-sm text-muted-foreground">
            Select a movie above to see real-time vector similarity search in
            action
          </p>
        </div>
      )}
    </div>
  );
}
