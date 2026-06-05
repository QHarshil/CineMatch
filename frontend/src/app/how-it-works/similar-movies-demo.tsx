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
    <div className="border border-border bg-wash p-6 sm:p-8">
      <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label
            htmlFor="seed-movie"
            className="mb-2 block text-sm text-muted-foreground"
          >
            Choose a title to find its nearest neighbors
          </label>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <select
              id="seed-movie"
              value={selectedId}
              onChange={(e) => handleSelect(e.target.value)}
              className="w-full cursor-pointer appearance-none border border-border bg-background py-2.5 pl-10 pr-4 text-sm text-foreground transition-colors focus:border-primary focus:outline-none"
            >
              <option value="">Select a title...</option>
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
          <Loader2 className="size-5 animate-spin text-primary" />
          <span className="ml-3 text-sm text-muted-foreground">
            Searching embedding space...
          </span>
        </div>
      )}

      {!loading && searched && neighbors.length > 0 && (
        <div className="space-y-6">
          {/* Seed movie label */}
          {selected && (
            <div className="flex items-center gap-3 border-b border-border pb-4">
              {selected.poster_path && (
                <Image
                  src={`https://image.tmdb.org/t/p/w92${selected.poster_path}`}
                  alt={selected.title}
                  width={36}
                  height={54}
                  className="border border-border object-cover"
                />
              )}
              <div>
                <p className="eyebrow text-muted-foreground">Seed title</p>
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
                className="flex items-center gap-4 border border-border bg-card px-4 py-3 transition-all duration-300 ease-out"
                style={{
                  animationDelay: `${i * 80}ms`,
                  animation: "fadeSlideIn 0.4s ease-out both",
                }}
              >
                <span className="w-5 shrink-0 font-mono text-xs text-muted-foreground">
                  #{i + 1}
                </span>
                {n.poster_path && (
                  <Image
                    src={`https://image.tmdb.org/t/p/w92${n.poster_path}`}
                    alt={n.title}
                    width={32}
                    height={48}
                    className="shrink-0 border border-border object-cover"
                  />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate font-heading text-base font-medium">
                    {n.title}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {n.genres?.slice(0, 3).join(", ")}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="font-mono text-sm text-primary">
                    {(n.similarity * 100).toFixed(1)}%
                  </p>
                  <p className="text-[10px] text-muted-foreground">similarity</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {!loading && !searched && (
        <div className="py-12 text-center">
          <p className="text-sm text-muted-foreground">
            Select a title above to see real-time vector similarity search in
            action
          </p>
        </div>
      )}
    </div>
  );
}
