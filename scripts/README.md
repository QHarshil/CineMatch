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

**Prerequisite:** apply the migrations in `migrations/` before seeding TV: `0002_add_media_type.sql` (the `media_type` column + `(tmdb_id, media_type)` unique index, since TMDB movie and TV IDs are separate namespaces) and `0003_add_original_language.sql` (stores `original_language` for the language filter).

**What it does:**
1. Fetches titles from TMDB discover (`/discover/movie` and/or `/discover/tv`), 20 per page, sorted by popularity or release date
2. Maps TMDB genre IDs to names using `/genre/movie/list` and `/genre/tv/list`
3. Generates 1536-dim embeddings via OpenAI `text-embedding-3-small` (5 concurrent workers, 80 RPM)
4. Upserts into Supabase in batches of 50, deduplicated by `(tmdb_id, media_type)`

**Expected runtime:** 3-5 minutes per ~500 titles (mostly OpenAI rate limiting).

**Required env vars:** `TMDB_READ_ACCESS_TOKEN`, `OPENAI_API_KEY`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY`

Rate limiting: 260ms delay between TMDB requests (under 40 req/10s), 80 RPM for OpenAI (under Tier-1's 100 RPM).

## Monthly freshness (.github/workflows/refresh-catalog.yml)

A scheduled GitHub Actions workflow runs the seeder in `recent` mode **monthly** (English + Korean) to ingest new releases (upserts, so no duplicates). Add the four env vars above as repository secrets (Settings > Secrets and variables > Actions), then it runs automatically or on demand via "Run workflow" in the Actions tab.

## backfill_details.ts

Fills `runtime`, `original_language`, and `backdrop_path` from the TMDB movie and TV detail endpoints for rows missing any of them. TMDB discover results have no runtime, so the seeder cannot set it. The refresh workflow runs this right after seeding.

```bash
cd scripts
npm ci
node backfill_details.ts            # Node 24+ runs TypeScript directly
node backfill_details.ts --dry-run  # fetch from TMDB, skip the writes
npm run typecheck && npm test
```

Only empty fields are written, so stored values are never overwritten. Requests are spaced 260ms apart across 4 workers (under TMDB's 40 per 10 seconds).

**Required env vars:** `TMDB_READ_ACCESS_TOKEN`, `SUPABASE_URL`, `SUPABASE_SECRET_KEY`
