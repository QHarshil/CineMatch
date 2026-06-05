#!/usr/bin/env bash
# Deploy the Python ranker to Cloud Run. Run from the repo root:
#   bash deploy/cloudrun-ranker.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REGION="${REGION:-us-central1}"

gcloud run deploy cinematch-ranker \
  --source "$REPO_ROOT/ranker" \
  --region "$REGION" \
  --allow-unauthenticated \
  --memory 512Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 2 \
  --timeout 30 \
  --set-env-vars APP_ENV=production
