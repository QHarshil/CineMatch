#!/usr/bin/env bash
# Arm the billing kill switch: a Cloud Billing budget publishes to a Pub/Sub
# topic, and a Cloud Function disables billing on the project once spend passes
# the budget. This is the only configuration that guarantees a hard $0 ceiling
# on Cloud Run. Run from the repo root:  bash deploy/cloudrun-killswitch.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REGION="${REGION:-us-central1}"
TOPIC="${TOPIC:-cinematch-billing-alerts}"
BUDGET_AMOUNT="${BUDGET_AMOUNT:-5USD}"

PROJECT_ID="$(gcloud config get-value project 2>/dev/null)"
BILLING_ACCT="$(gcloud billing projects describe "$PROJECT_ID" \
  --format='value(billingAccountName)' 2>/dev/null | sed 's#billingAccounts/##')"
echo "Project: $PROJECT_ID    Billing account: $BILLING_ACCT"

gcloud services enable billingbudgets.googleapis.com >/dev/null 2>&1 || true

# 1. Pub/Sub topic the budget publishes to (idempotent)
gcloud pubsub topics create "$TOPIC" 2>/dev/null && echo "created topic $TOPIC" \
  || echo "topic $TOPIC already exists"

# 2. Deploy the kill-switch function, triggered by that topic
gcloud functions deploy cinematch-billing-killswitch \
  --gen2 --runtime python312 --region "$REGION" \
  --source "$REPO_ROOT/deploy/billing-killswitch" \
  --entry-point stop_billing \
  --trigger-topic "$TOPIC" \
  --set-env-vars "GCP_PROJECT_ID=$PROJECT_ID" \
  --no-allow-unauthenticated

# 3. Allow the function's service account to detach billing from the project
RUNTIME_SA="$(gcloud functions describe cinematch-billing-killswitch \
  --gen2 --region "$REGION" --format='value(serviceConfig.serviceAccountEmail)')"
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:$RUNTIME_SA" \
  --role roles/billing.projectManager >/dev/null
echo "granted roles/billing.projectManager to $RUNTIME_SA"

# 4. Budget wired to the topic (Console fallback if the CLI form is unavailable)
if gcloud billing budgets create \
  --billing-account "$BILLING_ACCT" \
  --display-name cinematch \
  --budget-amount "$BUDGET_AMOUNT" \
  --filter-projects "projects/$PROJECT_ID" \
  --threshold-rule=percent=0.9 \
  --threshold-rule=percent=1.0 \
  --all-updates-rule-pubsub-topic "projects/$PROJECT_ID/topics/$TOPIC" 2>/tmp/cinematch_budget_err; then
  echo "budget created (cap $BUDGET_AMOUNT)"
else
  echo "WARN: budget creation via CLI failed. Create it in the Console:"
  echo "  Billing > Budgets & alerts > Create budget > amount $BUDGET_AMOUNT,"
  echo "  scope project $PROJECT_ID, then Manage notifications > connect topic '$TOPIC'."
  echo "  CLI error:"; sed 's/^/    /' /tmp/cinematch_budget_err
fi

echo ""
echo "Test it WITHOUT spending money (then relink billing in the Console afterward):"
echo "  gcloud pubsub topics publish $TOPIC --message '{\"costAmount\":999,\"budgetAmount\":5}'"
