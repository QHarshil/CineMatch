-- Base schema: catalog, users, interactions, ratings, and taste vectors, with
-- RLS on every table and the Stage-1 retrieval RPC. Later migrations extend it.

create extension if not exists vector;
create extension if not exists "uuid-ossp";
create extension if not exists pg_trgm;
create extension if not exists pgcrypto with schema extensions;

create table public.movies (
  id            uuid primary key default uuid_generate_v4(),
  tmdb_id       integer unique not null,
  title         text not null,
  overview      text,
  genres        text[] not null default '{}',
  release_year  integer,
  poster_path   text,
  backdrop_path text,
  vote_average  numeric(3,1),
  popularity    numeric(10,3),
  runtime       integer,
  embedding     vector(1536),
  created_at    timestamptz not null default now()
);

-- Mirrors auth.users and stores only a SHA-256 hash of the email.
create table public.users (
  id         uuid primary key references auth.users (id) on delete cascade,
  email_hash text unique not null,
  created_at timestamptz not null default now()
);

create table public.interactions (
  id         uuid primary key default uuid_generate_v4(),
  user_id    uuid not null references public.users (id) on delete cascade,
  movie_id   uuid not null references public.movies (id) on delete cascade,
  type       text not null check (type in ('like', 'dislike', 'watch', 'skip')),
  created_at timestamptz not null default now()
);

create table public.ratings (
  id         uuid primary key default gen_random_uuid(),
  user_id    uuid not null references public.users (id) on delete cascade,
  movie_id   uuid not null references public.movies (id) on delete cascade,
  score      smallint not null check (score between 1 and 10),
  created_at timestamptz default now(),
  unique (user_id, movie_id)
);

-- The Go backend rewrites a user's taste vector after each like or watch.
create table public.user_embeddings (
  user_id    uuid primary key references public.users (id) on delete cascade,
  embedding  vector(1536) not null,
  updated_at timestamptz not null default now()
);

create index movies_embedding_hnsw on public.movies
  using hnsw (embedding vector_cosine_ops) with (m = 16, ef_construction = 64);
create index user_embeddings_hnsw on public.user_embeddings
  using hnsw (embedding vector_cosine_ops) with (m = 16, ef_construction = 64);
create index movies_title_trgm on public.movies using gin (title gin_trgm_ops);
create index interactions_user_id_idx on public.interactions (user_id);
create index interactions_movie_id_idx on public.interactions (movie_id);
create index interactions_created_at_idx on public.interactions (created_at desc);
create unique index interactions_user_movie_type_unique on public.interactions (user_id, movie_id, type);

alter table public.movies enable row level security;
alter table public.users enable row level security;
alter table public.interactions enable row level security;
alter table public.ratings enable row level security;
alter table public.user_embeddings enable row level security;

-- The catalog is public; only the service role (seeder) writes it.
create policy movies_public_read on public.movies for select using (true);

create policy users_select_own on public.users for select using (auth.uid() = id);
create policy users_insert_own on public.users for insert with check (auth.uid() = id);
create policy users_update_own on public.users for update using (auth.uid() = id);

create policy interactions_select_own on public.interactions for select using (auth.uid() = user_id);
create policy interactions_insert_own on public.interactions for insert with check (auth.uid() = user_id);
create policy interactions_update_own on public.interactions for update using (auth.uid() = user_id);
create policy interactions_delete_own on public.interactions for delete using (auth.uid() = user_id);
-- Database-level backstop for the backend's 500-interaction cap.
create policy interactions_insert_cap on public.interactions for insert
  with check ((select count(*) from public.interactions where user_id = auth.uid()) < 500);

create policy ratings_select_own on public.ratings for select using (auth.uid() = user_id);
create policy ratings_insert_own on public.ratings for insert with check (auth.uid() = user_id);
create policy ratings_update_own on public.ratings for update using (auth.uid() = user_id);
create policy ratings_delete_own on public.ratings for delete using (auth.uid() = user_id);

create policy user_embeddings_select_own on public.user_embeddings for select using (auth.uid() = user_id);

-- A public.users row with the hashed email is created for every sign-up.
-- 0009 revises this for guest sessions, which have no email.
create function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path to ''
as $$
begin
  insert into public.users (id, email_hash)
  values (new.id, encode(extensions.digest(coalesce(new.email, ''), 'sha256'), 'hex'))
  on conflict (id) do nothing;
  return new;
end;
$$;

create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();

-- Stage-1 retrieval: the nearest titles to a query vector. 0005 adds
-- columns and switches it to security invoker.
create function public.match_movies(query_embedding vector(1536), match_count integer default 50)
returns table (
  id uuid, tmdb_id integer, title text, overview text, genres text[],
  release_year integer, poster_path text, vote_average numeric,
  popularity numeric, runtime integer, similarity float
)
language sql
stable
security definer
as $$
  select
    m.id, m.tmdb_id, m.title, m.overview, m.genres, m.release_year,
    m.poster_path, m.vote_average, m.popularity, m.runtime,
    1 - (m.embedding <=> query_embedding) as similarity
  from public.movies m
  where m.embedding is not null
  order by m.embedding <=> query_embedding
  limit match_count;
$$;
