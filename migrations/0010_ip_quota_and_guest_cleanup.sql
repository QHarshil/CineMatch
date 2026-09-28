-- Per-network assistant quota. Guest sessions are free to create, so a
-- per-account limit alone lets one visitor spend the shared daily budget.
-- Runs record a keyed hash of the client IP (never the IP itself), and usage
-- counts runs per hash.
alter table public.assistant_runs add column if not exists ip_hash text;

create index if not exists assistant_runs_ip_created_idx
  on public.assistant_runs (ip_hash, created_at desc);

drop function if exists public.assistant_usage(uuid, timestamptz);

create function public.assistant_usage(p_user_id uuid, p_ip_hash text, p_since timestamptz)
returns table (user_runs bigint, ip_runs bigint, total_runs bigint, total_tokens bigint)
language sql
stable
security definer
set search_path = ''
as $$
  select
    count(*) filter (where r.user_id = p_user_id),
    count(*) filter (where r.ip_hash = p_ip_hash),
    count(*),
    coalesce(sum(r.input_tokens + r.output_tokens), 0)
  from public.assistant_runs r
  where r.created_at >= p_since;
$$;

revoke execute on function public.assistant_usage from public, anon, authenticated;
grant execute on function public.assistant_usage to service_role;

-- A guest account cannot be recovered once its browser session is gone, so
-- guests idle for 30 days are deleted. Everything they own cascades from
-- public.users. The monthly refresh workflow calls this.
create or replace function public.delete_stale_guests(p_older_than interval default interval '30 days')
returns integer
language plpgsql
security definer
set search_path = ''
as $$
declare
  removed integer;
begin
  delete from auth.users u
  where u.is_anonymous
    and coalesce(u.last_sign_in_at, u.created_at) < now() - p_older_than;
  get diagnostics removed = row_count;
  return removed;
end;
$$;

revoke execute on function public.delete_stale_guests from public, anon, authenticated;
grant execute on function public.delete_stale_guests to service_role;
