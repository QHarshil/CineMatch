import { ScrollRow } from "@/components/scroll-row";
import type { Movie } from "@/types/movie";

export function SimilarMoviesRow({ movies }: { movies: Movie[] }) {
  return <ScrollRow title="Similar Movies" movies={movies} />;
}
