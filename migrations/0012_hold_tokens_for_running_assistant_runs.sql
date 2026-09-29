-- A reserved run starts with 0 tokens, so runs in flight did not count toward
-- the global token cap, and a run whose final write failed never counted.
-- The reservation now records a token hold (the per-run cap) that the
-- finished run overwrites with its real usage. p_token_hold defaults to 0 so
-- a backend still sending the old arguments keeps working until it redeploys.
drop function if exists public.reserve_assistant_run(uuid, uuid, text, timestamptz, integer, integer, text, text, text, text, integer);

create function public.reserve_assistant_run(
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
  p_input_chars integer,
  p_token_hold integer default 0
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
    (id, user_id, request_id, prompt_version, model, status, input_sha256, input_chars, ip_hash, latency_ms, input_tokens)
  values
    (p_run_id, p_user_id, p_request_id, p_prompt_version, p_model, 'running', p_input_sha256, p_input_chars, p_ip_hash, 0, greatest(p_token_hold, 0));

  return query select true, null::text, u, n, t, tok;
end;
$$;

revoke execute on function public.reserve_assistant_run from public, anon, authenticated;
grant execute on function public.reserve_assistant_run to service_role;
