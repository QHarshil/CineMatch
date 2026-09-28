/**
 * Offline retrieval eval for search_titles_hybrid.
 *
 * Two query sets, each run under three retrieval modes:
 *   keyword   full-text + trigram title matching (no query vector)
 *   semantic  pgvector cosine similarity only
 *   hybrid    all three fused with reciprocal rank fusion (production)
 *
 * Title lookups score MRR@10 and Hit@1 against the exact title, including
 * typos. Description queries are paraphrases that avoid genre words; a result
 * is relevant when its TMDB genres include the target genre. That label comes
 * from metadata the rankers never see, so no mode is graded on its own signal.
 *
 * Usage: node search.eval.ts   (needs SUPABASE_URL, SUPABASE_SECRET_KEY, OPENAI_API_KEY)
 */

import { callRpc, embed, selectRows, writeReport } from "./env.ts";
import { mean, ndcgAt, precisionAt, reciprocalRank, round } from "./metrics.ts";

interface TitleCase {
  query: string;
  title: string;
}

interface DescriptionCase {
  query: string;
  genres: string[];
}

const TITLE_CASES: TitleCase[] = [
  { query: "the dark knight", title: "The Dark Knight" },
  { query: "interstellar", title: "Interstellar" },
  { query: "parasite", title: "Parasite" },
  { query: "spirited away", title: "Spirited Away" },
  { query: "oldboy", title: "Oldboy" },
  { query: "breaking bad", title: "Breaking Bad" },
  { query: "squid game", title: "Squid Game" },
  { query: "the martian", title: "The Martian" },
  { query: "whiplash", title: "Whiplash" },
  { query: "my neighbor totoro", title: "My Neighbor Totoro" },
  // Typos and partial names.
  { query: "intersteller", title: "Interstellar" },
  { query: "the dark night", title: "The Dark Knight" },
  { query: "stranger thngs", title: "Stranger Things" },
  { query: "game of throne", title: "Game of Thrones" },
  { query: "la la lnd", title: "La La Land" },
];

const SCIFI = ["Science Fiction", "Sci-Fi & Fantasy"];
const FANTASY = ["Fantasy", "Sci-Fi & Fantasy"];

const DESCRIPTION_CASES: DescriptionCase[] = [
  { query: "something that will scare me at night", genres: ["Horror"] },
  { query: "a haunted house with a vengeful spirit", genres: ["Horror"] },
  { query: "laugh out loud and light hearted", genres: ["Comedy"] },
  { query: "real people and real events, filmed as they happened", genres: ["Documentary"] },
  { query: "falling in love against the odds", genres: ["Romance"] },
  { query: "detectives hunting a serial killer", genres: ["Crime", "Mystery", "Thriller"] },
  { query: "a crew planning the perfect robbery", genres: ["Crime", "Thriller"] },
  { query: "robots and artificial intelligence", genres: SCIFI },
  { query: "starships battling across the galaxy", genres: SCIFI },
  { query: "wizards, dragons, and magic kingdoms", genres: FANTASY },
  { query: "a cartoon adventure for young children", genres: ["Animation", "Family", "Kids"] },
  { query: "soldiers on the front line", genres: ["War", "War & Politics"] },
  { query: "cowboys and gunfights on the frontier", genres: ["Western"] },
  { query: "superheroes saving the world", genres: ["Action", "Action & Adventure"] },
  { query: "musicians chasing fame on tour", genres: ["Music"] },
  { query: "kings, queens, and palace intrigue centuries ago", genres: ["History", "War & Politics"] },
  { query: "strangers competing on an unscripted dating show", genres: ["Reality"] },
  { query: "a whodunit with a twist ending", genres: ["Mystery", "Crime"] },
];

type Mode = "keyword" | "semantic" | "hybrid";
const MODES: Mode[] = ["keyword", "semantic", "hybrid"];

interface Hit {
  id: string;
  title: string;
  genres: string[];
}

async function search(query: string, vector: number[], mode: Mode): Promise<Hit[]> {
  const payload: Record<string, unknown> = { query_text: query, match_count: 10 };
  if (mode !== "keyword") payload.query_embedding = vector;
  if (mode === "semantic") {
    payload.keyword_weight = 0;
    payload.title_weight = 0;
  }
  return callRpc<Hit[]>("search_titles_hybrid", payload);
}

async function main(): Promise<void> {
  const catalog = await selectRows<Hit>("movies", "select=id,title,genres");
  const idsByTitle = new Map<string, string[]>();
  for (const m of catalog) {
    const key = m.title.toLowerCase();
    idsByTitle.set(key, [...(idsByTitle.get(key) ?? []), m.id]);
  }

  const titleScores: Record<Mode, { rr: number[]; hit1: number[] }> = {
    keyword: { rr: [], hit1: [] },
    semantic: { rr: [], hit1: [] },
    hybrid: { rr: [], hit1: [] },
  };
  const titleRows: unknown[] = [];
  for (const c of TITLE_CASES) {
    const relevant = new Set(idsByTitle.get(c.title.toLowerCase()) ?? []);
    if (relevant.size === 0) throw new Error(`"${c.title}" is not in the catalog`);
    const vector = await embed(c.query);
    const row: Record<string, unknown> = { query: c.query, expected: c.title };
    for (const mode of MODES) {
      const ranked = (await search(c.query, vector, mode)).map((h) => h.id);
      const rr = reciprocalRank(ranked, relevant);
      titleScores[mode].rr.push(rr);
      titleScores[mode].hit1.push(rr === 1 ? 1 : 0);
      row[mode] = round(rr);
    }
    titleRows.push(row);
  }

  const descScores: Record<Mode, { p10: number[]; ndcg: number[] }> = {
    keyword: { p10: [], ndcg: [] },
    semantic: { p10: [], ndcg: [] },
    hybrid: { p10: [], ndcg: [] },
  };
  const baseRates: number[] = [];
  const descRows: unknown[] = [];
  for (const c of DESCRIPTION_CASES) {
    const target = new Set(c.genres);
    const relevant = new Set(catalog.filter((m) => m.genres.some((g) => target.has(g))).map((m) => m.id));
    baseRates.push(relevant.size / catalog.length);
    const vector = await embed(c.query);
    const row: Record<string, unknown> = { query: c.query, genres: c.genres, base_rate: round(relevant.size / catalog.length) };
    for (const mode of MODES) {
      const ranked = (await search(c.query, vector, mode)).map((h) => h.id);
      const p10 = precisionAt(ranked, relevant);
      descScores[mode].p10.push(p10);
      descScores[mode].ndcg.push(ndcgAt(ranked, relevant));
      row[mode] = round(p10, 2);
    }
    descRows.push(row);
  }

  const summary = MODES.map((mode) => ({
    mode,
    title_mrr10: round(mean(titleScores[mode].rr)),
    title_hit1: round(mean(titleScores[mode].hit1)),
    description_p10: round(mean(descScores[mode].p10)),
    description_ndcg10: round(mean(descScores[mode].ndcg)),
  }));

  console.log(`Catalog: ${catalog.length} titles. ${TITLE_CASES.length} title queries, ${DESCRIPTION_CASES.length} description queries.\n`);
  console.log("| Mode | Title MRR@10 | Title Hit@1 | Description P@10 | Description nDCG@10 |");
  console.log("|------|--------------|-------------|------------------|---------------------|");
  for (const s of summary) {
    console.log(`| ${s.mode} | ${s.title_mrr10} | ${s.title_hit1} | ${s.description_p10} | ${s.description_ndcg10} |`);
  }
  console.log(`\nRandom-ranking baseline for description P@10 (genre base rate): ${round(mean(baseRates))}`);

  const path = writeReport("search", {
    ran_at: new Date().toISOString(),
    catalog_size: catalog.length,
    summary,
    random_baseline_p10: round(mean(baseRates)),
    titles: titleRows,
    descriptions: descRows,
  });
  console.log(`Report: ${path}`);
}

main().catch((err: Error) => {
  console.error(err.message);
  process.exit(1);
});
