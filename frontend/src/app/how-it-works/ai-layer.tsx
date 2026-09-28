import { ASSISTANT_EVAL, SEARCH_EVAL } from "@/lib/eval-results";

const LOOP = [
  { step: "Request", body: "Up to 12 turns, validated and length-capped. Quota and budget checked first; the check fails closed." },
  { step: "Plan", body: "An OpenAI-compatible model chooses read-only tools. Constraints become filters." },
  { step: "Tools", body: "search_catalog, find_similar, get_taste_profile, get_recommendations. Each result title gets a short ref." },
  { step: "Ground", body: "present_picks may only cite refs a tool returned. Anything else is rejected and counted." },
  { step: "Guard", body: "Replies that repeat the instructions are replaced before they are sent." },
  { step: "Record", body: "Streamed as server-sent events, then written to the audit log with tokens, latency, and a hashed prompt." },
];

const GUARDRAILS = [
  ["Read-only tools", "The agent cannot write. Likes change only when the person clicks."],
  ["Grounding by ref", "Picks are validated against tool results on the server, not trusted from the model."],
  ["Output guard", "Tool names, headings, and any eight-word run of the instructions are blocked."],
  ["Budgets", "Per-user and guest daily limits, a global token cap, and an embedding cap. Past the cap, answers come from search."],
  ["Privacy", "Prompts are stored as SHA-256 hashes. Emails and phone numbers are masked in logged tool arguments."],
  ["Degradation", "No model, a rate limit, or an outage returns hybrid search results with a notice, never an empty screen."],
];

/**
 * The AI layer: the agent loop, its guardrails, and the retrieval and agent
 * eval results with the method behind them.
 */
export function AiLayer() {
  const best = {
    mrr: Math.max(...SEARCH_EVAL.modes.map((m) => m.titleMrr)),
    p10: Math.max(...SEARCH_EVAL.modes.map((m) => m.descriptionP10)),
  };

  return (
    <div>
      <p className="mb-10 max-w-2xl leading-relaxed text-muted-foreground">
        On top of the recommender sits a tool-calling agent. It plans with a language model but answers only from the
        catalog, and every step it takes is streamed to the page and written to an audit log. The model is swappable:
        the same Go client talks to Ollama on a laptop or a hosted provider in production.
      </p>

      <ol className="mb-14 grid gap-px border border-border bg-border sm:grid-cols-2 lg:grid-cols-3">
        {LOOP.map((item, i) => (
          <li key={item.step} className="bg-background p-5">
            <p className="font-mono text-xs text-primary">
              {String(i + 1).padStart(2, "0")} {item.step}
            </p>
            <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{item.body}</p>
          </li>
        ))}
      </ol>

      <h3 className="mb-2 font-heading text-xl font-semibold uppercase tracking-tight">Retrieval eval</h3>
      <p className="mb-6 max-w-2xl text-sm leading-relaxed text-muted-foreground">
        {SEARCH_EVAL.titleQueries} title lookups (including typos) and {SEARCH_EVAL.descriptionQueries} paraphrased
        descriptions that avoid genre words, over {SEARCH_EVAL.catalogSize.toLocaleString("en-US")} titles. A description
        result counts as relevant when its TMDB genres match the target, a label none of the rankers read. A random
        ranking scores {SEARCH_EVAL.randomP10.toFixed(2)} P@10.
      </p>
      <div className="mb-14 overflow-x-auto border border-border">
        <table className="w-full min-w-[560px] text-sm">
          <thead>
            <tr className="border-b border-border bg-wash">
              <th className="eyebrow px-5 py-3 text-left text-muted-foreground">Retrieval</th>
              <th className="eyebrow px-5 py-3 text-right text-muted-foreground">Title MRR@10</th>
              <th className="eyebrow px-5 py-3 text-right text-muted-foreground">Title Hit@1</th>
              <th className="eyebrow px-5 py-3 text-right text-muted-foreground">Description P@10</th>
              <th className="eyebrow px-5 py-3 text-right text-muted-foreground">nDCG@10</th>
            </tr>
          </thead>
          <tbody className="font-mono">
            {SEARCH_EVAL.modes.map((m) => (
              <tr key={m.mode} className="border-b border-border last:border-b-0">
                <td className={`px-5 py-3 font-sans ${m.mode === "Hybrid (production)" ? "font-medium text-foreground" : "text-muted-foreground"}`}>
                  {m.mode}
                </td>
                <td className={`px-5 py-3 text-right ${m.titleMrr === best.mrr ? "text-primary" : "text-muted-foreground"}`}>{m.titleMrr.toFixed(3)}</td>
                <td className="px-5 py-3 text-right text-muted-foreground">{m.titleHit1.toFixed(3)}</td>
                <td className={`px-5 py-3 text-right ${m.descriptionP10 === best.p10 ? "text-primary" : "text-muted-foreground"}`}>
                  {m.descriptionP10.toFixed(3)}
                </td>
                <td className="px-5 py-3 text-right text-muted-foreground">{m.descriptionNdcg.toFixed(3)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <h3 className="mb-2 font-heading text-xl font-semibold uppercase tracking-tight">Agent eval</h3>
      <p className="mb-6 max-w-2xl text-sm leading-relaxed text-muted-foreground">
        {ASSISTANT_EVAL.cases} cases through the real API on {ASSISTANT_EVAL.model}, run locally with Ollama. Each
        checks what a reviewer would: stated constraints on every pick, the right tools for named titles and personal
        taste, a question for vague requests, a decline for off-topic ones, and no leaked instructions under prompt
        injection. {ASSISTANT_EVAL.note}
      </p>
      <div className="mb-14 grid gap-px border border-border bg-border lg:grid-cols-2">
        <table className="bg-background text-sm">
          <tbody>
            {ASSISTANT_EVAL.categories.map((c) => (
              <tr key={c.name} className="border-b border-border last:border-b-0">
                <td className="px-5 py-2.5 text-muted-foreground">{c.name}</td>
                <td className="px-5 py-2.5 text-right font-mono text-foreground">
                  {c.passed}/{c.cases}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <dl className="grid grid-cols-2 gap-px bg-border">
          {ASSISTANT_EVAL.summary.map(([label, value]) => (
            <div key={label} className="bg-background px-5 py-3">
              <dt className="eyebrow text-muted-foreground">{label}</dt>
              <dd className="mt-1 font-mono text-lg text-foreground">{value}</dd>
            </div>
          ))}
        </dl>
      </div>

      <h3 className="mb-6 font-heading text-xl font-semibold uppercase tracking-tight">Guardrails</h3>
      <dl className="grid gap-px border border-border bg-border sm:grid-cols-2">
        {GUARDRAILS.map(([name, body]) => (
          <div key={name} className="bg-background p-5">
            <dt className="font-mono text-sm text-primary">{name}</dt>
            <dd className="mt-1.5 text-sm leading-relaxed text-muted-foreground">{body}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
