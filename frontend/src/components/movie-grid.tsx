import { Film } from "lucide-react";
import type { Movie } from "@/types/movie";
import { MovieCard } from "@/components/movie-card";

interface MovieGridProps {
  movies: Movie[];
}

export function MovieGrid({ movies }: MovieGridProps) {
  if (movies.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-4">
        <div className="w-16 h-16 border border-border flex items-center justify-center">
          <Film className="w-7 h-7 text-muted-foreground" strokeWidth={1.5} />
        </div>
        <p className="font-heading text-xl font-semibold">No movies found</p>
        <p className="text-sm text-muted-foreground">
          Try adjusting your filters or search term.
        </p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-x-4 gap-y-8">
      {movies.map((movie) => (
        <MovieCard key={movie.id} movie={movie} />
      ))}
    </div>
  );
}
