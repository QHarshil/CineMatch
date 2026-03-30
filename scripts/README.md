# CineMatch Scripts

Data pipeline scripts for populating and maintaining the movie database.

## seed_movies.go

Populates the Supabase `movies` table with TMDB movie data and OpenAI embeddings. This is the script that builds the initial movie catalog.

```bash
cd scripts
go run seed_movies.go
```

Dry run (fetches from TMDB but skips the database write):

```bash
go run seed_movies.go --dry-run
```

**What it does:**
1. Fetches 500 movies from TMDB's discover endpoint (25 pages of 20, sorted by popularity)
2. Maps TMDB genre IDs to names using the `/genre/movie/list` endpoint (1 request)
3. Generates 1536-dim embeddings via OpenAI `text-embedding-3-small` (5 concurrent workers, rate limited to 80 RPM)
4. Upserts into Supabase in batches of 50 (deduplicates by `tmdb_id`)

**Expected runtime:** 3-5 minutes (mostly waiting on OpenAI rate limits).

**Required env vars:**
- `TMDB_READ_ACCESS_TOKEN`
- `OPENAI_API_KEY`
- `SUPABASE_URL`
- `SUPABASE_SECRET_KEY`

Rate limiting: 260ms delay between TMDB requests (stays under 40 req/10s limit), 80 RPM for OpenAI (safely under Tier-1's 100 RPM limit).

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

**Expected runtime:** 1-2 minutes for 494 movies (300ms delay between TMDB requests).

**Required env vars:**
- `TMDB_READ_ACCESS_TOKEN`
- `SUPABASE_URL`
- `SUPABASE_SECRET_KEY`
