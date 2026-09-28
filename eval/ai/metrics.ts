/** Ranking and latency metrics shared by the search and assistant evals. */

/** Reciprocal rank of the first relevant result within k, or 0. */
export function reciprocalRank(ranked: string[], relevant: Set<string>, k = 10): number {
  const index = ranked.slice(0, k).findIndex((id) => relevant.has(id));
  return index === -1 ? 0 : 1 / (index + 1);
}

/** Fraction of the top k that is relevant. Short lists count missing slots as misses. */
export function precisionAt(ranked: string[], relevant: Set<string>, k = 10): number {
  const hits = ranked.slice(0, k).filter((id) => relevant.has(id)).length;
  return hits / k;
}

/** Binary-relevance nDCG@k: DCG of the ranking over the best achievable DCG. */
export function ndcgAt(ranked: string[], relevant: Set<string>, k = 10): number {
  const dcg = ranked
    .slice(0, k)
    .reduce((sum, id, i) => sum + (relevant.has(id) ? 1 / Math.log2(i + 2) : 0), 0);
  const ideal = Array.from({ length: Math.min(k, relevant.size) }).reduce<number>(
    (sum, _, i) => sum + 1 / Math.log2(i + 2),
    0,
  );
  return ideal === 0 ? 0 : dcg / ideal;
}

/** Nearest-rank percentile, p in [0, 100]. */
export function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const rank = Math.ceil((p / 100) * sorted.length);
  return sorted[Math.min(sorted.length, Math.max(1, rank)) - 1];
}

export function mean(values: number[]): number {
  return values.length === 0 ? 0 : values.reduce((a, b) => a + b, 0) / values.length;
}

export function round(value: number, digits = 3): number {
  const f = 10 ** digits;
  return Math.round(value * f) / f;
}
