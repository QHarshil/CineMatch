#!/usr/bin/env node
/**
 * Backfills backdrop_path for all movies in the Supabase movies table
 * by fetching from the TMDB API.
 *
 * Usage: node scripts/backfill_backdrop.mjs
 * Requires: TMDB_READ_ACCESS_TOKEN, SUPABASE_URL, SUPABASE_SECRET_KEY in .env
 */

import { readFileSync } from "fs";

// Parse .env manually (no dotenv dependency)
const envFile = readFileSync(new URL("../.env", import.meta.url), "utf-8");
const env = {};
for (const line of envFile.split("\n")) {
  const match = line.match(/^([^#=]+)=(.*)$/);
  if (match) env[match[1].trim()] = match[2].trim();
}

const TMDB_TOKEN = env.TMDB_READ_ACCESS_TOKEN;
const SUPABASE_URL = env.SUPABASE_URL;
const SUPABASE_KEY = env.SUPABASE_SECRET_KEY;

if (!TMDB_TOKEN || !SUPABASE_URL || !SUPABASE_KEY) {
  console.error("Missing required env vars");
  process.exit(1);
}

const TMDB_BASE = "https://api.themoviedb.org/3";
const BATCH_SIZE = 20;
const DELAY_MS = 300; // respect TMDB rate limits

async function fetchAllMovies() {
  const res = await fetch(
    `${SUPABASE_URL}/rest/v1/movies?backdrop_path=is.null&select=tmdb_id&order=tmdb_id&limit=1000`,
    {
      headers: {
        apikey: SUPABASE_KEY,
        Authorization: `Bearer ${SUPABASE_KEY}`,
      },
    }
  );
  if (!res.ok) throw new Error(`Supabase fetch failed: ${res.status}`);
  return res.json();
}

async function fetchTmdbBackdrop(tmdbId) {
  const res = await fetch(`${TMDB_BASE}/movie/${tmdbId}?language=en-US`, {
    headers: { Authorization: `Bearer ${TMDB_TOKEN}` },
  });
  if (!res.ok) return null;
  const data = await res.json();
  return data.backdrop_path || null;
}

async function updateBackdropPath(tmdbId, backdropPath) {
  const res = await fetch(
    `${SUPABASE_URL}/rest/v1/movies?tmdb_id=eq.${tmdbId}`,
    {
      method: "PATCH",
      headers: {
        apikey: SUPABASE_KEY,
        Authorization: `Bearer ${SUPABASE_KEY}`,
        "Content-Type": "application/json",
        Prefer: "return=minimal",
      },
      body: JSON.stringify({ backdrop_path: backdropPath }),
    }
  );
  if (!res.ok) {
    console.error(`  Failed to update tmdb_id=${tmdbId}: ${res.status}`);
  }
}

async function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function main() {
  const movies = await fetchAllMovies();
  console.log(`Found ${movies.length} movies missing backdrop_path`);

  let updated = 0;
  let skipped = 0;

  for (let i = 0; i < movies.length; i++) {
    const { tmdb_id } = movies[i];
    const backdropPath = await fetchTmdbBackdrop(tmdb_id);

    if (backdropPath) {
      await updateBackdropPath(tmdb_id, backdropPath);
      updated++;
    } else {
      skipped++;
    }

    if ((i + 1) % BATCH_SIZE === 0) {
      console.log(
        `  Progress: ${i + 1}/${movies.length} (updated: ${updated}, no backdrop: ${skipped})`
      );
      await sleep(DELAY_MS);
    }
  }

  console.log(
    `Done. Updated: ${updated}, No backdrop available: ${skipped}, Total: ${movies.length}`
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
