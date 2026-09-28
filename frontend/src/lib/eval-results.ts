/**
 * Measured eval results shown on the site. Update these from the reports the
 * eval scripts print; nothing here is estimated.
 *   Ranker:    eval/eval_rankers.py, eval/benchmark_latency.py
 *   Retrieval: eval/ai/search.eval.ts
 *   Agent:     eval/ai/assistant.eval.ts
 */

export const RANKER_EVAL = {
  lambdamartNdcg10: 0.814,
  liftOverPopularity: 14,
  rerankP95Ms: 0.9,
};

export const SEARCH_EVAL = {
  ranAt: "2026-09-28",
  catalogSize: 1843,
  titleQueries: 15,
  descriptionQueries: 18,
  randomP10: 0.152,
  modes: [
    { mode: "Keyword only", titleMrr: 1, titleHit1: 1, descriptionP10: 0.011, descriptionNdcg: 0.02 },
    { mode: "Semantic only", titleMrr: 0.967, titleHit1: 0.933, descriptionP10: 0.739, descriptionNdcg: 0.726 },
    { mode: "Hybrid (production)", titleMrr: 1, titleHit1: 1, descriptionP10: 0.739, descriptionNdcg: 0.726 },
  ],
};

export const ASSISTANT_EVAL = {
  ranAt: "2026-09-28",
  model: "qwen3:8b",
  cases: 23,
  passed: 23,
  note: "Before the output guard existed, one injection case leaked part of the instructions (22/23); the guard blocked it and one other attempt in the final run.",
  categories: [
    { name: "Hard constraints", passed: 10, cases: 10 },
    { name: "Like a named title", passed: 4, cases: 4 },
    { name: "From personal taste", passed: 2, cases: 2 },
    { name: "Vague, asks a question", passed: 2, cases: 2 },
    { name: "Off-topic, declines", passed: 2, cases: 2 },
    { name: "Prompt injection", passed: 3, cases: 3 },
  ],
  summary: [
    ["Picks meeting constraints", "66/66"],
    ["Ungrounded picks shown", "0"],
    ["Mean tool calls", "1.57"],
    ["Tokens per run", "6,274"],
    ["Latency p50", "10.8 s"],
    ["Latency p95", "19.3 s"],
  ] as const,
};
