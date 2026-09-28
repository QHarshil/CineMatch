-- Count an assistant run when it starts. Checking usage first and writing the
-- audit row after the run let parallel requests all pass the check, and a
-- failed final write left a run uncounted. The reservation counts and inserts
-- under per-network and per-user advisory locks; the backend then updates the
-- row with the outcome.
alter table public.assistant_runs drop constraint if exists assistant_runs_status_check;
alter table public.assistant_runs add constraint assistant_runs_status_check
  check (status in ('running', 'picks', 'answered', 'fallback', 'error'));

create or replace function public.reserve_assistant_run(
  p_run_id uuid,
  p_user_id uuid,
  p_ip_hash text,
  p_since timestamptz,
  p_user_limit integer,
  p_ip_limit integer,
  p_request_id text,
  p_prompt_version text,
  p_model text,
  p_input_sha256 text,
  p_input_chars integer
)
returns table (allowed boolean, reason text, user_runs bigint, ip_runs bigint, total_runs bigint, total_tokens bigint)
language plpgsql
security definer
set search_path = ''
as $$
declare
  u bigint;
  n bigint;
  t bigint;
  tok bigint;
begin
  -- Always network first, then user, so two reservations cannot deadlock.
  perform pg_advisory_xact_lock(hashtextextended('assistant-ip:' || p_ip_hash, 0));
  perform pg_advisory_xact_lock(hashtextextended('assistant-user:' || p_user_id::text, 0));

  select
    count(*) filter (where r.user_id = p_user_id),
    count(*) filter (where r.ip_hash = p_ip_hash),
    count(*),
    coalesce(sum(r.input_tokens + r.output_tokens), 0)
  into u, n, t, tok
  from public.assistant_runs r
  where r.created_at >= p_since;

  if u >= p_user_limit then
    return query select false, 'user'::text, u, n, t, tok;
    return;
  end if;
  if n >= p_ip_limit then
    return query select false, 'network'::text, u, n, t, tok;
    return;
  end if;

  insert into public.assistant_runs
    (id, user_id, request_id, prompt_version, model, status, input_sha256, input_chars, ip_hash, latency_ms)
  values
    (p_run_id, p_user_id, p_request_id, p_prompt_version, p_model, 'running', p_input_sha256, p_input_chars, p_ip_hash, 0);

  return query select true, null::text, u, n, t, tok;
end;
$$;

revoke execute on function public.reserve_assistant_run from public, anon, authenticated;
grant execute on function public.reserve_assistant_run to service_role;

-- Judge a guest idle by its sessions, not last_sign_in_at, which token
-- refreshes do not update. A 7-day floor stops a stray argument from
-- deleting every guest.
create or replace function public.delete_stale_guests(p_older_than interval default interval '30 days')
returns integer
language plpgsql
security definer
set search_path = ''
as $$
declare
  cutoff timestamptz := now() - greatest(p_older_than, interval '7 days');
  removed integer;
begin
  delete from auth.users u
  where u.is_anonymous
    and u.created_at < cutoff
    and not exists (
      select 1
      from auth.sessions s
      where s.user_id = u.id
        and greatest(s.created_at, s.updated_at, s.refreshed_at at time zone 'UTC') >= cutoff
    );
  get diagnostics removed = row_count;
  return removed;
end;
$$;

revoke execute on function public.delete_stale_guests from public, anon, authenticated;
grant execute on function public.delete_stale_guests to service_role;
