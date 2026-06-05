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

Cloud Run builds the `ranker/Dockerfile` from source. The trained model ships in
`ranker/model/lambdamart-v1.txt`, so the container can load it with no extra
config.

```bash
cd ranker
gcloud run deploy cinematch-ranker \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --memory 512Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 2 \
  --timeout 30 \
  --set-env-vars APP_ENV=production
```

Copy the service URL it prints (looks like
`https://cinematch-ranker-XXXXXXXX-uc.a.run.app`). That is `RANKER_URL` below.

Verify:

```bash
curl https://cinematch-ranker-XXXXXXXX-uc.a.run.app/health
# {"status":"ok","service":"cinematch-ranker"}
```

## 2. Deploy the Go backend

Secrets are passed through a local env file so they never land in shell history.
Create `backend/.env.cloudrun.yaml` (already gitignored):

```yaml
JWT_SECRET: "your-supabase-jwt-secret"
SUPABASE_URL: "https://YOUR_PROJECT.supabase.co"
SUPABASE_SECRET_KEY: "your-supabase-service-role-key"
RANKER_URL: "https://cinematch-ranker-XXXXXXXX-uc.a.run.app"
ALLOWED_ORIGINS: "https://your-frontend-domain.com"
RATE_LIMIT_RPM: "60"
```

Deploy:

```bash
cd ../backend
gcloud run deploy cinematch-backend \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 3 \
  --timeout 30 \
  --env-vars-file .env.cloudrun.yaml
```

Copy the backend URL it prints. Verify it reaches Supabase:

```bash
curl https://cinematch-backend-XXXXXXXX-uc.a.run.app/health
# status "ok", database "reachable", plus movie_count / user_count / interaction_count
```

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
  containers can run at once. This is the main cost ceiling: a traffic flood is
  rejected with 429/503 rather than scaling into a large bill.
- `--min-instances 0` means no charge while idle.
- `--timeout 30` stops a single slow request from accruing minutes of CPU.
- The Go API already rate-limits (60 req/min per IP, tighter per endpoint), so
  abusive traffic is answered with cheap 429s.

These caps bound cost in real time. The backstop that guarantees a hard ceiling
is a budget that disables billing when breached. Run all of this against the
dedicated CineMatch project.

The script `deploy/cloudrun-killswitch.sh` performs steps 1 to 5 below in one
go (and falls back to Console instructions for the budget if the CLI form is
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
  --threshold-rule percent=0.5 \
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

This requires the backend to attach a Google-signed identity token to ranker
requests, which is a follow-up code change in `ranker/client.go`.

## No-card alternative: Render

Render needs no credit card but spins services down after 15 minutes of
inactivity, so the first request takes 30 to 60 seconds. Create two Web Services
from this repo, set the root directory to `backend/` and `ranker/`, runtime
Docker, and the same environment variables as above. Render injects `PORT`,
which both services already honor.
