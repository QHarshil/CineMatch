import type { Movie } from "@/types/movie";

export const arrival: Movie = {
  id: "7f1c2a4e-0000-4000-8000-000000000001",
  tmdb_id: 329865,
  media_type: "movie",
  title: "Arrival",
  overview: "A linguist works with the military to communicate with alien lifeforms.",
  genres: ["Science Fiction", "Drama"],
  release_year: 2016,
  poster_path: "/x2FJsf1ElAgr63Y3PNPtJrcmpoe.jpg",
  backdrop_path: "/yIZ1xendyqKvY3FGeeUYUd5X9Mm.jpg",
  vote_average: 7.6,
  popularity: 54.2,
  runtime: 116,
};

export const darkSeries: Movie = {
  ...arrival,
  id: "7f1c2a4e-0000-4000-8000-000000000002",
  tmdb_id: 70523,
  media_type: "tv",
  title: "Dark",
  genres: ["Mystery", "Drama"],
  release_year: 2017,
  vote_average: 8.4,
};
