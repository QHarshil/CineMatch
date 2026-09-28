-- For each recommended title, the title the user liked or watched that is
-- closest in embedding space. Powers "because you liked X" explanations
-- without shipping any vectors to the backend.

create or replace function public.nearest_liked_titles(p_user_id uuid, p_movie_ids uuid[])
returns table (movie_id uuid, liked_id uuid, liked_title text, similarity float8)
language sql
stable
security definer
set search_path = public, extensions
as $$
  with liked as (
    select m.id, m.title, m.embedding
    from public.movies m
    where m.embedding is not null
      and m.id in (
        select i.movie_id
        from public.interactions i
        where i.user_id = p_user_id and i.type in ('like', 'watch')
        order by i.created_at desc
        limit 100
      )
  )
  select c.id, best.id, best.title, best.similarity
  from public.movies c
  cross join lateral (
    select l.id, l.title, 1 - (c.embedding <=> l.embedding) as similarity
    from liked l
    where l.id <> c.id
    order by c.embedding <=> l.embedding
    limit 1
  ) best
  where c.id = any (p_movie_ids) and c.embedding is not null;
$$;

-- Reads another user's interactions, so only the backend may call it.
revoke execute on function public.nearest_liked_titles from public, anon, authenticated;
grant execute on function public.nearest_liked_titles to service_role;
