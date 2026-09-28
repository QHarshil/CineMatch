-- Audit log for the assistant. One row per run records what the agent did
-- (tools called, titles presented, grounding drops), which model and prompt
-- version produced it, and its token cost. Prompts are stored only as a
-- SHA-256 hash and a length, keeping plaintext user input out of the database.
-- The same rows back per-user quotas and the global daily token budget.

create table if not exists public.assistant_runs (
  id uuid primary key,
  user_id uuid not null references public.users (id) on delete cascade,
  created_at timestamptz not null default now(),
  request_id text,
  prompt_version text not null,
  model text not null,
  status text not null check (status in ('picks', 'answered', 'fallback', 'error')),
  input_sha256 text not null,
  input_chars integer not null,
  steps jsonb not null default '[]'::jsonb,
  pick_ids uuid[] not null default '{}',
  ungrounded_dropped integer not null default 0,
  input_tokens integer not null default 0,
  output_tokens integer not null default 0,
  latency_ms integer not null
);

create index if not exists assistant_runs_user_created_idx
  on public.assistant_runs (user_id, created_at desc);
create index if not exists assistant_runs_created_idx
  on public.assistant_runs (created_at desc);

-- Users can read their own history. Writes come only from the Go backend's
-- service role, so there are no insert, update, or delete policies.
alter table public.assistant_runs enable row level security;

create policy assistant_runs_select_own on public.assistant_runs
  for select using ((select auth.uid()) = user_id);

-- Usage since a cutoff (the backend passes UTC midnight): this user's runs,
-- all runs, and all tokens. One round trip covers the per-user quota and the
-- global budget.
create or replace function public.assistant_usage(p_user_id uuid, p_since timestamptz)
returns table (user_runs bigint, total_runs bigint, total_tokens bigint)
language sql
stable
security definer
set search_path = ''
as $$
  select
    count(*) filter (where r.user_id = p_user_id),
    count(*),
    coalesce(sum(r.input_tokens + r.output_tokens), 0)
  from public.assistant_runs r
  where r.created_at >= p_since;
$$;

revoke execute on function public.assistant_usage from public, anon, authenticated;
grant execute on function public.assistant_usage to service_role;
