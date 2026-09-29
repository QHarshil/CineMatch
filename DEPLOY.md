# Deploying CineMatch

The frontend runs on Vercel. The Go API and the Python ranker run as two
containers. This guide deploys both containers to Google Cloud Run, which has a
request-based always-free tier that scales to zero, so a low-traffic portfolio
app costs nothing. A no-card Render alternative is at the end.

```
Vercel (frontend)  ->  Cloud Run: cinematch-backend (Go)  ->  Cloud Run: cinematch-ranker (Python)
                                       |
                                       +-> Supabase (Postgres + pgvector)
```

The ranker is stateless and holds no secrets or data access: it takes candidate
movies plus user features and returns scores. Deploying it with public ingress
is acceptable for this project. To lock it down later, see "Hardening" below.

## Prerequisites

- The `gcloud` CLI, authenticated against a project with billing enabled
  (the free tier still requires a billing account on file):

  ```bash
  gcloud auth login
  gcloud config set project YOUR_PROJECT_ID
  gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com
  ```

- Region `us-central1` is used below (Cloud Run free-tier eligible).

## 1. Deploy the ranker

From the repo root:

```bash
bash deploy/cloudrun-ranker.sh
```

Cloud Run builds `ranker/Dockerfile` from source with 512 MiB, at most 2
instances, a 30-second timeout, and `APP_ENV=production`. The trained model
ships in `ranker/model/lambdamart-v1.txt`, so the container needs no extra
config.

Copy the service URL it prints (looks like
`https://cinematch-ranker-XXXXXXXX-uc.a.run.app`). That is `RANKER_URL` below.

Verify:

```bash
curl https://cinematch-ranker-XXXXXXXX-uc.a.run.app/health
# {"status":"ok","service":"cinematch-ranker"}
```

## 2. Deploy the Go backend

`deploy/cloudrun-backend.sh` reads secrets from the local `.env`, passes them
to Cloud Run in a temporary file outside the uploaded source, and deploys with a
120-second request timeout so assistant streams can finish. Optional keys are included only when
set, so each feature stays off without them.

```bash
ALLOWED_ORIGINS=https://cinematch.harshilc.com bash deploy/cloudrun-backend.sh
```

| `.env` key | Effect in production |
|------------|----------------------|
| `OPENAI_API_KEY` | Embedding search on `/discover`; keyword-only without it |
| `DEPLOY_LLM_BASE_URL`, `DEPLOY_LLM_MODEL`, `DEPLOY_LLM_API_KEY` | The assistant model; search-only answers without them |
| `DEPLOY_LLM_REASONING_EFFORT` | Set only if the provider accepts `reasoning_effort` |
| `ASSISTANT_*_DAILY_*`, `EMBED_DAILY_LIMIT` | Budgets; match them to the provider's free tier |
| `OMDB_API_KEY` | IMDb and Rotten Tomatoes ratings |

`LLM_*` in `.env` points at a local Ollama for development, so production reads
the separate `DEPLOY_LLM_*` keys.

Verify it reaches Supabase:

```bash
curl https://cinematch-backend-XXXXXXXX-uc.a.run.app/health
# status "ok", database "ok", plus movie_count / user_count / interaction_count
```

### Choosing a free model provider

Ollama runs the model on the machine that hosts it, so visitors cannot reach a
laptop's Ollama, and a GPU on Cloud Run is not free. A hosted free tier with an
OpenAI-compatible endpoint works with no code changes. As of September 2026:

| Provider | `DEPLOY_LLM_BASE_URL` | Free tier | Fit |
|----------|-----------------------|-----------|-----|
| Google Gemini (Flash-Lite) | `https://generativelanguage.googleapis.com/v1beta/openai` | 15 requests/min, 1,000/day | Best daily capacity. An assistant run makes 2 to 3 model calls, so about 350 runs/day. Free-tier prompts may be used to improve Google's products. |
| Groq (gpt-oss-20b, Qwen) | `https://api.groq.com/openai/v1` | 30 requests/min, 1,000/day, 8,000 tokens/min, 200,000 tokens/day | Fastest, but runs average about 6,300 tokens, so roughly one run a minute and 30 a day. |

Set the global caps just under the provider's limits: for Gemini Flash-Lite,
`ASSISTANT_DAILY_RUNS=350`; for Groq, `ASSISTANT_DAILY_TOKENS=180000`. Past a
cap, or on a provider 429, the assistant answers from search and says so.

### Guest sessions

The assistant offers "Try it as a guest", which uses Supabase anonymous sign-in.
Enable it in the Supabase dashboard under Authentication > Sign In / Providers >
"Allow anonymous sign-ins". Migration `0009_guest_users.sql` must be applied
first so guests do not collide on `email_hash`. Guests get
`ASSISTANT_GUEST_DAILY_RUNS` (default 8). Supabase rate-limits anonymous
sign-ins per IP, and the global caps bound total spend.

For magic links to return to the page that asked for sign-in, the Supabase
redirect allow list needs a wildcard entry such as
`https://cinematch.harshilc.com/**`.

### Order matters

The frontend calls `/discover` and `/assistant`, so deploy the backend (and
apply migrations `0004` through `0011`) before the frontend build that uses
them goes live.

## 3. Point the frontend at the new backend

In the Vercel project settings, set the environment variable and redeploy
(`NEXT_PUBLIC_*` values are inlined at build time, so a redeploy is required):

```bash
# from the frontend/ directory, or set it in the Vercel dashboard
vercel env add NEXT_PUBLIC_API_URL production
# value: https://cinematch-backend-XXXXXXXX-uc.a.run.app
vercel --prod
```

`next.config.ts` reads `NEXT_PUBLIC_API_URL` into the CSP `connect-src`, so the
browser is allowed to call the backend once this is set and rebuilt.

## 4. End-to-end check

1. Open your deployed frontend and run a search. Search hits the Go backend, so
   a result list confirms the backend is reachable.
2. Sign in, like a few movies, open For You. A personalized list confirms the
   full two-stage path (retrieval, then ranker) is live.

## Keeping it warm

Cloud Run scales to zero, so the first request after an idle period pays a one
to three second cold start. Because the free tier is request-based, a cheap way
to avoid that during the day is a free uptime monitor (UptimeRobot,
cron-job.org) pinging `/health` on both services every 10 minutes. Do not set
`--min-instances 1`: a pinned warm instance is billed and would leave the free
tier.

## Cost protection

Cloud Run bills pay-as-you-go with no built-in hard cap, so the deploy commands
above include the guardrails that bound spend:

- `--max-instances` (2 for the ranker, 3 for the backend) caps how many
  containers can run at once. This is the main cost ceiling: past it, a traffic
  flood gets 429/503 responses and no new containers start.
- `--min-instances 0` means no charge while idle.
- Request timeouts (30 s for the ranker, 120 s for the backend so assistant
  streams can finish) stop a single slow request from accruing minutes of CPU.
- The Go API already rate-limits (60 req/min per IP, tighter per endpoint), so
  abusive traffic is answered with cheap 429s.

These caps bound cost in real time. The backstop that guarantees a hard ceiling
is a budget that disables billing when breached. Run all of this against the
dedicated CineMatch project.

After enabling the APIs in step 1, `deploy/cloudrun-killswitch.sh` runs steps
2 to 5 (and prints Console instructions for the budget if the CLI form is
unavailable). The manual steps follow for reference.

### 1. Enable the APIs the kill switch needs

```bash
gcloud services enable \
  cloudbilling.googleapis.com cloudfunctions.googleapis.com \
  pubsub.googleapis.com eventarc.googleapis.com run.googleapis.com \
  cloudbuild.googleapis.com artifactregistry.googleapis.com
```

### 2. Create the Pub/Sub topic the budget publishes to

```bash
gcloud pubsub topics create cinematch-billing-alerts
```

### 3. Create the budget, wired to that topic

```bash
gcloud billing budgets create \
  --billing-account YOUR_BILLING_ACCOUNT_ID \
  --display-name cinematch \
  --budget-amount 5USD \
  --filter-projects projects/YOUR_PROJECT_ID \
  --threshold-rule percent=0.9 \
  --threshold-rule percent=1.0 \
  --all-updates-rule-pubsub-topic projects/YOUR_PROJECT_ID/topics/cinematch-billing-alerts
```

If `gcloud billing budgets` is unavailable, create the budget in the Console
under Billing > Budgets & alerts and connect the same Pub/Sub topic there.

### 4. Deploy the kill-switch function

The source lives in `deploy/billing-killswitch/`.

```bash
gcloud functions deploy cinematch-billing-killswitch \
  --gen2 \
  --runtime python312 \
  --region us-central1 \
  --source deploy/billing-killswitch \
  --entry-point stop_billing \
  --trigger-topic cinematch-billing-alerts \
  --set-env-vars GCP_PROJECT_ID=YOUR_PROJECT_ID \
  --no-allow-unauthenticated
```

### 5. Let the function disable billing

Grant the function's runtime service account permission to detach billing from
the project:

```bash
RUNTIME_SA=$(gcloud functions describe cinematch-billing-killswitch \
  --gen2 --region us-central1 --format 'value(serviceConfig.serviceAccountEmail)')

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member "serviceAccount:${RUNTIME_SA}" \
  --role roles/billing.projectManager
```

If the function logs a permission error when it fires, also grant that service
account `roles/billing.admin` on the billing account.

### Notes

- `--max-instances` is the real-time cap; the kill switch is the backstop.
  Budget data refreshes a few times a day, so the kill switch can lag by hours.
  The two together keep worst-case spend tiny.
- Disabling billing stops the whole project, which is why CineMatch should be
  its own project. Re-enabling billing is manual: relink the billing account in
  the Console when you want to bring it back.
- Test the switch without spending real money by publishing a fake breach:

  ```bash
  gcloud pubsub topics publish cinematch-billing-alerts \
    --message '{"costAmount":999,"budgetAmount":5}'
  ```

  Check the function logs, then relink billing if it disabled the project.

## Hardening (optional)

Make the ranker private and let only the backend call it:

```bash
gcloud run services update cinematch-ranker --region us-central1 --no-allow-unauthenticated
gcloud run services add-iam-policy-binding cinematch-ranker \
  --region us-central1 \
  --member "serviceAccount:BACKEND_SERVICE_ACCOUNT" \
  --role roles/run.invoker
```

The backend would then need to send a Google-signed identity token with each
ranker request (`backend/ranker/client.go`). It does not today, so the ranker
stays public and only re-scores the candidates it is sent.

## No-card alternative: Render

Render needs no credit card but spins services down after 15 minutes of
inactivity, so the first request takes 30 to 60 seconds. Create two Web Services
from this repo, set the root directory to `backend/` and `ranker/`, runtime
Docker, and the same environment variables as above. Render injects `PORT`,
which both services already honor.
