/**
 * Fills runtime, original_language, and backdrop_path from TMDB detail
 * endpoints for catalog rows missing any of them. TMDB discover results carry
 * no runtime, so the seeder cannot set it; this runs after each seed.
 *
 * Usage: node scripts/backfill_details.ts [--dry-run]
 * Env: TMDB_READ_ACCESS_TOKEN, SUPABASE_URL, SUPABASE_SECRET_KEY
 */

const TMDB_BASE = "https://api.themoviedb.org/3";
// TMDB allows 40 requests per 10 seconds; 260ms spacing stays under it.
const TMDB_SPACING_MS = 260;
const WORKERS = 4;
const PAGE_SIZE = 1000;

interface CatalogRow {
  id: string;
  tmdb_id: number;
  media_type: "movie" | "tv";
  title: string;
  runtime: number | null;
  original_language: string | null;
  backdrop_path: string | null;
}

interface TmdbDetails {
  runtime?: number | null;
  episode_run_time?: number[];
  last_episode_to_air?: { runtime?: number | null } | null;
  original_language?: string;
  backdrop_path?: string | null;
}

type DetailPatch = Partial<Pick<CatalogRow, "runtime" | "original_language" | "backdrop_path">>;

interface Config {
  tmdbToken: string;
  supabaseUrl: string;
  supabaseKey: string;
  dryRun: boolean;
}

function loadConfig(): Config {
  try {
    process.loadEnvFile(new URL("../.env", import.meta.url));
  } catch {
    // CI passes the variables directly; a missing .env is expected there.
  }
  const tmdbToken = process.env.TMDB_READ_ACCESS_TOKEN ?? "";
  const supabaseUrl = (process.env.SUPABASE_URL ?? "").replace(/\/$/, "");
  const supabaseKey = process.env.SUPABASE_SECRET_KEY ?? "";
  if (!tmdbToken || !supabaseUrl || !supabaseKey) {
    throw new Error("TMDB_READ_ACCESS_TOKEN, SUPABASE_URL, and SUPABASE_SECRET_KEY are required");
  }
  return { tmdbToken, supabaseUrl, supabaseKey, dryRun: process.argv.includes("--dry-run") };
}

function supabaseHeaders(config: Config): Record<string, string> {
  return {
    apikey: config.supabaseKey,
    Authorization: `Bearer ${config.supabaseKey}`,
    "Content-Type": "application/json",
  };
}

async function fetchIncompleteRows(config: Config): Promise<CatalogRow[]> {
  const rows: CatalogRow[] = [];
  const filter = "or=(runtime.is.null,runtime.eq.0,original_language.is.null,backdrop_path.is.null)";
  const select = "select=id,tmdb_id,media_type,title,runtime,original_language,backdrop_path";
  for (let offset = 0; ; offset += PAGE_SIZE) {
    const url = `${config.supabaseUrl}/rest/v1/movies?${select}&${filter}&order=id&limit=${PAGE_SIZE}&offset=${offset}`;
    const res = await fetch(url, { headers: supabaseHeaders(config) });
    if (!res.ok) throw new Error(`listing incomplete rows: HTTP ${res.status}`);
    const page = (await res.json()) as CatalogRow[];
    rows.push(...page);
    if (page.length < PAGE_SIZE) return rows;
  }
}

/** Spaces calls at least `spacingMs` apart across all workers. */
function createPacer(spacingMs: number): () => Promise<void> {
  let nextSlot = 0;
  return async () => {
    const now = Date.now();
    const wait = Math.max(0, nextSlot - now);
    nextSlot = Math.max(now, nextSlot) + spacingMs;
    if (wait > 0) await new Promise((resolve) => setTimeout(resolve, wait));
  };
}

async function fetchTmdbDetails(config: Config, row: CatalogRow): Promise<TmdbDetails | null> {
  const res = await fetch(`${TMDB_BASE}/${row.media_type}/${row.tmdb_id}?language=en-US`, {
    headers: { Authorization: `Bearer ${config.tmdbToken}` },
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`TMDB ${row.media_type}/${row.tmdb_id}: HTTP ${res.status}`);
  return (await res.json()) as TmdbDetails;
}

/** Series report per-episode runtimes; the first listed value is TMDB's typical length. */
export function runtimeFromDetails(details: TmdbDetails): number | null {
  const candidates = [
    details.runtime,
    details.episode_run_time?.[0],
    details.last_episode_to_air?.runtime,
  ];
  const minutes = candidates.find((value): value is number => typeof value === "number" && value > 0);
  return minutes ?? null;
}

/** Only fills fields that are missing, so curated values are never overwritten. */
export function buildPatch(row: CatalogRow, details: TmdbDetails): DetailPatch {
  const patch: DetailPatch = {};
  const runtime = runtimeFromDetails(details);
  if (!row.runtime && runtime) patch.runtime = runtime;
  if (!row.original_language && details.original_language) {
    patch.original_language = details.original_language;
  }
  if (!row.backdrop_path && details.backdrop_path) patch.backdrop_path = details.backdrop_path;
  return patch;
}

async function applyPatch(config: Config, row: CatalogRow, patch: DetailPatch): Promise<void> {
  const res = await fetch(`${config.supabaseUrl}/rest/v1/movies?id=eq.${row.id}`, {
    method: "PATCH",
    headers: { ...supabaseHeaders(config), Prefer: "return=minimal" },
    body: JSON.stringify(patch),
  });
  if (!res.ok) throw new Error(`updating ${row.id}: HTTP ${res.status}`);
}

async function main(): Promise<void> {
  const config = loadConfig();
  const rows = await fetchIncompleteRows(config);
  console.log(`${rows.length} titles missing runtime, language, or backdrop`);

  const pace = createPacer(TMDB_SPACING_MS);
  const counts = { updated: 0, unchanged: 0, notFound: 0, failed: 0 };
  let cursor = 0;

  async function worker(): Promise<void> {
    while (cursor < rows.length) {
      const row = rows[cursor++];
      try {
        await pace();
        const details = await fetchTmdbDetails(config, row);
        if (!details) {
          counts.notFound++;
          continue;
        }
        const patch = buildPatch(row, details);
        if (Object.keys(patch).length === 0) {
          counts.unchanged++;
          continue;
        }
        if (!config.dryRun) await applyPatch(config, row, patch);
        counts.updated++;
      } catch (err) {
        counts.failed++;
        console.error(`${row.title}: ${(err as Error).message}`);
      }
      const done = counts.updated + counts.unchanged + counts.notFound + counts.failed;
      if (done % 200 === 0) console.log(`${done}/${rows.length}`);
    }
  }

  await Promise.all(Array.from({ length: WORKERS }, worker));
  const verb = config.dryRun ? "would update" : "updated";
  console.log(
    `${verb} ${counts.updated}, unchanged ${counts.unchanged}, not on TMDB ${counts.notFound}, failed ${counts.failed}`,
  );
  if (counts.failed > 0) process.exitCode = 1;
}

if (import.meta.main) {
  main().catch((err: Error) => {
    console.error(err.message);
    process.exit(1);
  });
}
