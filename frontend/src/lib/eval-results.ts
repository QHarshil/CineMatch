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
  models: [
    { model: "Popularity Baseline", ndcg: 0.716, mrr: 0.875, hitRate: 1.0 },
    { model: "Vector Retrieval Only", ndcg: 0.798, mrr: 0.938, hitRate: 1.0 },
    { model: "Linear Re-ranker", ndcg: 0.795, mrr: 0.95, hitRate: 1.0 },
    { model: "LambdaMART Re-ranker", ndcg: 0.814, mrr: 0.988, hitRate: 1.0 },
  ],
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

/** One run of the 23-case agent eval against a given model. */
export interface AgentEvalRun {
  model: string;
  where: string;
  passed: number;
  cases: number;
  /** Passed per category, in AGENT_EVAL_CATEGORIES order. */
  byCategory: number[];
  picksMeetingConstraints: string;
  ungroundedCaught: number;
  meanToolCalls: number;
  tokensPerRun: number;
  latencyP50S: number;
  latencyP95S: number;
}

export const AGENT_EVAL_CATEGORIES = [
  { name: "Hard constraints", cases: 10 },
  { name: "Like a named title", cases: 4 },
  { name: "From personal taste", cases: 2 },
  { name: "Vague, asks a question", cases: 2 },
  { name: "Off-topic, declines", cases: 2 },
  { name: "Prompt injection", cases: 3 },
];

export const AGENT_EVAL = {
  ranAt: "2026-09-28",
  production: {
    model: "gemini-3.5-flash-lite",
    where: "production, Gemini free tier",
    passed: 22,
    cases: 23,
    byCategory: [10, 4, 2, 1, 2, 3],
    picksMeetingConstraints: "74/74",
    ungroundedCaught: 1,
    meanToolCalls: 1.17,
    tokensPerRun: 4223,
    latencyP50S: 3.3,
    latencyP95S: 5.9,
  } satisfies AgentEvalRun,
  local: {
    model: "qwen3:8b",
    where: "local, Ollama on a laptop",
    passed: 23,
    cases: 23,
    byCategory: [10, 4, 2, 2, 2, 3],
    picksMeetingConstraints: "66/66",
    ungroundedCaught: 0,
    meanToolCalls: 1.57,
    tokensPerRun: 6274,
    latencyP50S: 10.8,
    latencyP95S: 19.3,
  } satisfies AgentEvalRun,
  notes: [
    "Gemini's miss: asked \"any ideas?\", it recommended from the person's taste instead of asking a question.",
    "The grounding check caught one pick Gemini tried to cite without a tool result; it was never shown.",
    "qwen3:8b first scored 22/23: a developer-mode injection leaked part of its instructions, which led to the output guard.",
  ],
};
