-- Store TMDB original language so the catalog can be filtered or pruned to a
-- chosen set of languages (e.g. English + Korean). The seeder writes it via
-- --languages; existing rows stay NULL until the seeder next upserts them.
alter table movies add column if not exists original_language text;
