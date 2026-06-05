#!/usr/bin/env bash
# Deploy the Go backend to Cloud Run. Reads secrets from the local .env and
# writes the gitignored backend/.env.cloudrun.yaml, so secrets stay on your
# machine. Run from the repo root, after the ranker is deployed, passing your
# deployed frontend origin:
#   ALLOWED_ORIGINS=https://your-app.vercel.app bash deploy/cloudrun-backend.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REGION="${REGION:-us-central1}"

ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-}"
if [[ -z "$ALLOWED_ORIGINS" ]]; then
  echo "Set ALLOWED_ORIGINS to your frontend origin (no trailing slash), e.g.:" >&2
  echo "  ALLOWED_ORIGINS=https://your-app.vercel.app bash deploy/cloudrun-backend.sh" >&2
  exit 1
fi

# Auto-discover the ranker URL from Cloud Run if not passed explicitly.
RANKER_URL="${RANKER_URL:-$(gcloud run services describe cinematch-ranker \
  --region "$REGION" --format='value(status.url)' 2>/dev/null || true)}"
if [[ -z "$RANKER_URL" ]]; then
  echo "Could not find the ranker URL. Deploy the ranker first, or pass RANKER_URL=... explicitly." >&2
  exit 1
fi
echo "Using RANKER_URL=$RANKER_URL"

get() {
  local v
  v="$(grep -E "^$1=" "$REPO_ROOT/.env" | head -1 | cut -d= -f2-)"
  v="${v%\"}"; v="${v#\"}"
  printf '%s' "$v"
}

JWT_SECRET="$(get JWT_SECRET)"
SUPABASE_URL="$(get SUPABASE_URL)"
SUPABASE_SECRET_KEY="$(get SUPABASE_SECRET_KEY)"
OMDB_API_KEY="$(get OMDB_API_KEY)" # optional; enables IMDb/Rotten Tomatoes ratings

for name in JWT_SECRET SUPABASE_URL SUPABASE_SECRET_KEY; do
  if [[ -z "${!name}" ]]; then
    echo "Could not read $name from $REPO_ROOT/.env" >&2
    exit 1
  fi
done

ENV_FILE="$REPO_ROOT/backend/.env.cloudrun.yaml"
cat > "$ENV_FILE" <<EOF
JWT_SECRET: "$JWT_SECRET"
SUPABASE_URL: "$SUPABASE_URL"
SUPABASE_SECRET_KEY: "$SUPABASE_SECRET_KEY"
RANKER_URL: "$RANKER_URL"
ALLOWED_ORIGINS: "$ALLOWED_ORIGINS"
RATE_LIMIT_RPM: "60"
EOF
# OMDB_API_KEY is optional; only include it when set so ratings stay disabled otherwise.
if [[ -n "$OMDB_API_KEY" ]]; then
  echo "OMDB_API_KEY: \"$OMDB_API_KEY\"" >> "$ENV_FILE"
fi
echo "Wrote $ENV_FILE"

gcloud run deploy cinematch-backend \
  --source "$REPO_ROOT/backend" \
  --region "$REGION" \
  --allow-unauthenticated \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 3 \
  --timeout 30 \
  --env-vars-file "$ENV_FILE"
