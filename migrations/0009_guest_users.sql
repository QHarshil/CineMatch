-- Guest (anonymous) sessions have no email. Hashing an empty string gave every
-- guest the same email_hash and broke the unique constraint, so guests hash
-- their user ID instead. Still no plaintext PII either way.
create or replace function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path to ''
as $function$
begin
  insert into public.users (id, email_hash)
  values (
    new.id,
    encode(
      extensions.digest(
        case when coalesce(new.email, '') = '' then 'guest:' || new.id::text else new.email end,
        'sha256'
      ),
      'hex'
    )
  )
  on conflict (id) do nothing;
  return new;
end;
$function$;

-- The trigger runs regardless of EXECUTE grants; nobody should call it
-- through the REST API.
revoke execute on function public.handle_new_user() from public, anon, authenticated;
