import type { RankingFactor, RecommendationExplanation } from "@/types/movie";

const FACTOR_LABELS: Record<RankingFactor["feature"], string> = {
  similarity: "Close to your taste",
  vote_average: "Highly rated",
  log_popularity: "Popular now",
  decade: "An era you watch",
  is_recent: "Recent release",
};

/**
 * Why a title is in the feed: the liked title it sits closest to, and the
 * ranker features (SHAP) that pushed it up.
 */
export function PickExplanation({ explanation }: { explanation?: RecommendationExplanation }) {
  if (!explanation) return null;
  const liked = explanation.because_you_liked;
  const factors = explanation.factors ?? [];
  if (!liked && factors.length === 0) return null;

  return (
    <div className="mt-2 border-t border-border pt-2">
      {liked && (
        <p className="font-mono text-[10px] leading-snug text-muted-foreground">
          Because you liked <span className="text-primary">{liked.title}</span>
        </p>
      )}
      {factors.length > 0 && (
        <ul className="mt-1.5 flex flex-wrap gap-1" aria-label="Ranking factors">
          {factors.slice(0, 2).map((f) => (
            <li
              key={f.feature}
              title={`SHAP contribution +${f.contribution.toFixed(3)}`}
              className="border border-primary/30 bg-wash px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wider text-primary"
            >
              {FACTOR_LABELS[f.feature] ?? f.feature}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
