import { Film } from "lucide-react";
import type { Movie } from "@/types/movie";
import { MovieCard } from "@/components/movie-card";

interface MovieGridProps {
  movies: Movie[];
}

export function MovieGrid({ movies }: MovieGridProps) {
  if (movies.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-20">
        <div className="flex size-16 items-center justify-center border border-border text-primary">
          <Film className="size-7" strokeWidth={1.5} />
        </div>
        <p className="font-heading text-xl font-semibold uppercase tracking-tight">
          No titles found
        </p>
        <p className="font-serif text-muted-foreground">
          Try adjusting your filters or search term.
        </p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      {movies.map((movie) => (
        <MovieCard key={movie.id} movie={movie} />
      ))}
    </div>
  );
}
