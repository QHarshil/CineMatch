-- Return media_type, backdrop_path, and original_language from match_movies so
-- personalized results and similar-title lookups carry the Film/TV label and
-- language without a second query. The return type changes, so the function is
-- dropped and recreated. It runs as SECURITY INVOKER: movies is public-read
-- under RLS, so the anon-key demo route needs no elevated privileges.

drop function if exists public.match_movies(vector, integer);

create function public.match_movies(query_embedding vector, match_count integer default 50)
returns table (
  id uuid,
  tmdb_id integer,
  media_type text,
  title text,
  overview text,
  genres text[],
  release_year integer,
  poster_path text,
  backdrop_path text,
  vote_average numeric,
  popularity numeric,
  runtime integer,
  original_language text,
  similarity double precision
)
language sql
stable
security invoker
set search_path = public, extensions
as $$
  select
    m.id, m.tmdb_id, m.media_type, m.title, m.overview, m.genres,
    m.release_year, m.poster_path, m.backdrop_path, m.vote_average,
    m.popularity, m.runtime, m.original_language,
    1 - (m.embedding <=> query_embedding) as similarity
  from public.movies m
  where m.embedding is not null
  order by m.embedding <=> query_embedding
  limit match_count;
$$;
