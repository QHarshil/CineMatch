# CineMatch Scripts

Data pipeline scripts for populating and maintaining the movie database.

## seed_movies.go

Populates the Supabase `movies` table with TMDB data (movies and/or TV shows) and OpenAI embeddings. Builds the initial catalog and, in `recent` mode, ingests new releases.

```bash
cd scripts
go run seed_movies.go --media both --count 600                # initial catalog: 600 movies + 600 shows
go run seed_movies.go --media movie                           # movies only (default)
go run seed_movies.go --media both --mode recent --count 100  # newest releases (freshness cron)
go run seed_movies.go --dry-run                               # fetch + embed, skip the DB write
```

Flags: `--media` (`movie` | `tv` | `both`), `--mode` (`popular` | `recent`), `--count` (titles per media type, split across languages, default 500), `--languages` (comma-separated TMDB original-language codes, default `en,ko`), `--dry-run`.

**Prerequisite:** apply the migrations in `migrations/` before seeding TV — `0002_add_media_type.sql` (the `media_type` column + `(tmdb_id, media_type)` unique index, since TMDB movie and TV IDs are separate namespaces) and `0003_add_original_language.sql` (stores `original_language` for the language filter).

**What it does:**
1. Fetches titles from TMDB discover (`/discover/movie` and/or `/discover/tv`), 20 per page, sorted by popularity or release date
2. Maps TMDB genre IDs to names using `/genre/movie/list` and `/genre/tv/list`
3. Generates 1536-dim embeddings via OpenAI `text-embedding-3-small` (5 concurrent workers, 80 RPM)
4. Upserts into Supabase in batches of 50, deduplicated by `(tmdb_id, media_type)`

**Expected runtime:** 3-5 minutes per ~500 titles (mostly OpenAI rate limiting).

**Required env vars:** `TMDB_READ_ACCESS_TOKEN`, `OPENAI_API_KEY`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY`

Rate limiting: 260ms delay between TMDB requests (under 40 req/10s), 80 RPM for OpenAI (under Tier-1's 100 RPM).

## Weekly freshness (.github/workflows/refresh-catalog.yml)

A scheduled GitHub Actions workflow runs the seeder in `recent` mode **monthly** (English + Korean) to ingest new releases (upserts, so no duplicates). Add the four env vars above as repository secrets (Settings > Secrets and variables > Actions), then it runs automatically or on demand via "Run workflow" in the Actions tab.

## backfill_backdrop.mjs

Backfills the `backdrop_path` column for movies that are missing TMDB backdrop images. Run this after `seed_movies.go` if you need backdrop images for the movie detail pages.

```bash
node scripts/backfill_backdrop.mjs
```

**What it does:**
1. Queries Supabase for all movies where `backdrop_path IS NULL`
2. For each movie, fetches the backdrop from TMDB by `tmdb_id`
3. Updates the row in Supabase via REST PATCH
4. Prints a summary: updated count, skipped count (movies with no TMDB backdrop)

**Expected runtime:** a few minutes for the full catalog (300ms delay between TMDB requests).

**Required env vars:**
- `TMDB_READ_ACCESS_TOKEN`
- `SUPABASE_URL`
- `SUPABASE_SECRET_KEY`
