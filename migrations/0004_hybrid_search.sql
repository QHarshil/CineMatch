-- Hybrid retrieval for natural-language queries. Three rankers run over the
-- same filtered set and are fused with reciprocal rank fusion (RRF):
--   semantic: cosine distance between the query and title+overview embeddings
--   keyword:  Postgres full-text rank, title weighted above overview
--   title:    trigram similarity, which tolerates typos in title lookups
-- RRF uses ranks, not raw scores, so the three need no score calibration.

alter table public.movies
  add column if not exists search_document tsvector
  generated always as (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(overview, '')), 'B')
  ) stored;

create index if not exists movies_search_document_idx
  on public.movies using gin (search_document);

-- Load pgvector in this session so the hnsw.* settings used below are
-- registered when the function is created.
do $$ begin perform '[1]'::vector; end $$;

create or replace function public.search_titles_hybrid(
  query_text text,
  query_embedding vector(1536) default null,
  match_count int default 20,
  filter_media_type text default null,
  filter_genres text[] default null,
  min_year int default null,
  max_year int default null,
  min_rating numeric default null,
  max_runtime int default null,
  filter_language text default null,
  exclude_ids uuid[] default null,
  semantic_weight float8 default 1.0,
  keyword_weight float8 default 1.0,
  title_weight float8 default 1.0,
  rrf_k int default 60
)
returns table (
  id uuid,
  tmdb_id int,
  media_type text,
  title text,
  overview text,
  genres text[],
  release_year int,
  poster_path text,
  backdrop_path text,
  vote_average numeric,
  popularity numeric,
  runtime int,
  original_language text,
  similarity float8,
  semantic_rank int,
  keyword_rank int,
  title_rank int,
  score float8
)
language sql
stable
set search_path = public, extensions
-- Let the HNSW scan keep walking the graph until enough rows pass the
-- filters, so a strict filter still fills the page.
set hnsw.iterative_scan = relaxed_order
set hnsw.ef_search = 100
as $$
  -- Each branch repeats the filters so the planner can pick an index per
  -- branch. Sharing one filtered CTE would detoast every embedding.
  with semantic as materialized (
    select m.id, m.embedding <=> query_embedding as distance
    from public.movies m
    where query_embedding is not null
      and m.embedding is not null
      and (filter_media_type is null or m.media_type = filter_media_type)
      and (filter_genres is null or m.genres && filter_genres)
      and (min_year is null or m.release_year >= min_year)
      and (max_year is null or m.release_year <= max_year)
      and (min_rating is null or m.vote_average >= min_rating)
      and (max_runtime is null or (m.runtime > 0 and m.runtime <= max_runtime))
      and (filter_language is null or m.original_language = filter_language)
      and (exclude_ids is null or m.id <> all (exclude_ids))
    order by m.embedding <=> query_embedding
    limit least(match_count, 50) * 2
  ),
  semantic_ranked as (
    select id, 1 - distance as similarity, row_number() over (order by distance)::int as rank_ix
    from semantic
  ),
  keyword as (
    select m.id, row_number() over (order by ts_rank_cd(m.search_document, q) desc)::int as rank_ix
    from public.movies m, websearch_to_tsquery('english', query_text) q
    where m.search_document @@ q
      and (filter_media_type is null or m.media_type = filter_media_type)
      and (filter_genres is null or m.genres && filter_genres)
      and (min_year is null or m.release_year >= min_year)
      and (max_year is null or m.release_year <= max_year)
      and (min_rating is null or m.vote_average >= min_rating)
      and (max_runtime is null or (m.runtime > 0 and m.runtime <= max_runtime))
      and (filter_language is null or m.original_language = filter_language)
      and (exclude_ids is null or m.id <> all (exclude_ids))
    order by ts_rank_cd(m.search_document, q) desc
    limit least(match_count, 50) * 2
  ),
  title_match as (
    select m.id, row_number() over (order by similarity(m.title, query_text) desc)::int as rank_ix
    from public.movies m
    where similarity(m.title, query_text) >= 0.4
      and (filter_media_type is null or m.media_type = filter_media_type)
      and (filter_genres is null or m.genres && filter_genres)
      and (min_year is null or m.release_year >= min_year)
      and (max_year is null or m.release_year <= max_year)
      and (min_rating is null or m.vote_average >= min_rating)
      and (max_runtime is null or (m.runtime > 0 and m.runtime <= max_runtime))
      and (filter_language is null or m.original_language = filter_language)
      and (exclude_ids is null or m.id <> all (exclude_ids))
    order by similarity(m.title, query_text) desc
    limit least(match_count, 50)
  ),
  fused as (
    select
      coalesce(s.id, k.id, t.id) as id,
      s.similarity,
      s.rank_ix as semantic_rank,
      k.rank_ix as keyword_rank,
      t.rank_ix as title_rank,
      coalesce(semantic_weight / (rrf_k + s.rank_ix), 0)
        + coalesce(keyword_weight / (rrf_k + k.rank_ix), 0)
        + coalesce(title_weight / (rrf_k + t.rank_ix), 0) as score
    from semantic_ranked s
    full outer join keyword k on k.id = s.id
    full outer join title_match t on t.id = coalesce(s.id, k.id)
  )
  select
    m.id, m.tmdb_id, m.media_type, m.title, m.overview, m.genres,
    m.release_year, m.poster_path, m.backdrop_path, m.vote_average,
    m.popularity, m.runtime, m.original_language,
    fused.similarity, fused.semantic_rank, fused.keyword_rank, fused.title_rank,
    fused.score
  from fused
  join public.movies m on m.id = fused.id
  order by fused.score desc, m.popularity desc
  limit least(match_count, 50);
$$;

-- Only the Go backend (service role) calls this, so queries stay behind the
-- API's validation and rate limits.
revoke execute on function public.search_titles_hybrid from public, anon, authenticated;
grant execute on function public.search_titles_hybrid to service_role;
