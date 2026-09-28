-- Record when the assistant's output guard replaced a reply that repeated
-- its instructions, so blocked prompt-injection attempts show in the audit log.
alter table public.assistant_runs
  add column if not exists output_blocked boolean not null default false;
