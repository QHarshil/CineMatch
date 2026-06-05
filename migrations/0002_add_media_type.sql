-- Adds TV-show support to the shared movies catalog.
-- Apply via the Supabase SQL editor (Dashboard > SQL Editor) BEFORE seeding TV
-- or redeploying the backend with the media_type field.

-- 1. media_type distinguishes movies from TV shows. Existing rows default to
--    'movie', so the column is safe to add to a populated table.
alter table movies
  add column if not exists media_type text not null default 'movie'
  check (media_type in ('movie', 'tv'));

-- 2. TMDB movie IDs and TV IDs are separate namespaces, so tmdb_id is only
--    unique within a media type. Replace the single-column unique on tmdb_id
--    with a composite one (the seeder upserts on (tmdb_id, media_type)).
do $$
declare existing_constraint text;
begin
  select c.conname into existing_constraint
  from pg_constraint c
  where c.conrelid = 'movies'::regclass
    and c.contype = 'u'
    and array_length(c.conkey, 1) = 1
    and c.conkey[1] = (
      select attnum from pg_attribute
      where attrelid = 'movies'::regclass and attname = 'tmdb_id'
    );
  if existing_constraint is not null then
    execute format('alter table movies drop constraint %I', existing_constraint);
  end if;
end $$;

create unique index if not exists movies_tmdb_id_media_type_idx
  on movies (tmdb_id, media_type);

-- 3. Optional: let the catalog filter by type efficiently once TV rows exist.
create index if not exists movies_media_type_idx on movies (media_type);
