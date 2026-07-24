# Deploying MedMarket to GKE (staging)

First-bring-up runbook for the staging environment. The model and rationale live
in the [README's Deployment section](../README.md#deployment); this is the
step-by-step. All commands assume the GCP project
`project-8628faf2-7f2e-46d0-a01`, region `us-central1`, zonal cluster
`medmarket` in `us-central1-a`.

## Prerequisites

- `gcloud` authenticated: `gcloud auth login` **and** `gcloud auth application-default login`
- `terraform`, `kubectl`, `kustomize`, `docker`
- `atlas` (for the migration step)

## 1. Provision infrastructure

```sh
cd terraform
terraform init
terraform apply
```

Creates the GKE cluster + node pool (autoscale 0–2), Artifact Registry, the GCS
bucket, all service accounts and Workload Identity bindings (storage, collector,
CI), and enables the required APIs. Note two outputs for the next step:

```sh
terraform output ci_workload_identity_provider
terraform output ci_service_account_email
```

## 2. Configure GitHub (for CI deploys)

In **Settings → Secrets and variables → Actions**:

- **Variables:** `WIF_PROVIDER` and `CI_SERVICE_ACCOUNT` (the two outputs above).
- **Secrets:** `POSTGRES_PASSWORD`, `DATABASE_URL`, `JWT_SECRET`,
  `STRIPE_SECRET_KEY`, `PHARMACY_A_SECRET`, `PHARMACY_B_SECRET`. Generate fresh
  values for `JWT_SECRET` and `POSTGRES_PASSWORD` (`openssl rand -base64 48`);
  `DATABASE_URL`'s password must match `POSTGRES_PASSWORD`. Do not reuse the
  committed compose placeholders.

## 3. Deploy

**Option A — CI (recommended):** merge to `main` (or run the `Deploy Staging`
workflow via *Run workflow*). It builds/pushes the six images, pins the commit
SHA into the overlay, and applies.

**Option B — manual first deploy:**

```sh
gcloud auth configure-docker us-central1-docker.pkg.dev
REG=us-central1-docker.pkg.dev/project-8628faf2-7f2e-46d0-a01/medmarket
SHA=$(git rev-parse --short HEAD)

docker build -t "$REG/backend:$SHA"         --build-arg CMD=server ./backend
docker build -t "$REG/worker:$SHA"          --build-arg CMD=worker ./backend
docker build -t "$REG/migrate:$SHA"         -f ./backend/Dockerfile.migrate ./backend
docker build -t "$REG/frontend:$SHA"        -f ./frontend/Dockerfile.prod ./frontend
docker build -t "$REG/mock-pharmacy-a:$SHA" ./services/mock-pharmacy-a
docker build -t "$REG/mock-pharmacy-b:$SHA" ./services/mock-pharmacy-b
docker build -t "$REG/mock-shipping:$SHA"   ./services/mock-shipping
for i in backend worker migrate frontend mock-pharmacy-a mock-pharmacy-b mock-shipping; do docker push "$REG/$i:$SHA"; done

# Real secrets for the staging secretGenerator:
cp k8s/overlays/staging/.env.example k8s/overlays/staging/.env
$EDITOR k8s/overlays/staging/.env          # fill in real values

cd k8s/overlays/staging
kustomize edit set image \
  medmarket-backend="$REG/backend:$SHA" \
  medmarket-worker="$REG/worker:$SHA" \
  medmarket-migrate="$REG/migrate:$SHA" \
  medmarket-frontend="$REG/frontend:$SHA" \
  medmarket-mock-pharmacy-a="$REG/mock-pharmacy-a:$SHA" \
  medmarket-mock-pharmacy-b="$REG/mock-pharmacy-b:$SHA" \
  medmarket-mock-shipping="$REG/mock-shipping:$SHA"
cd -

gcloud container clusters get-credentials medmarket --zone us-central1-a
kubectl apply -k k8s/overlays/staging
```

Applying wakes the node pool from 0 — expect a few minutes while nodes provision
and pods pull images.

## 4. Database migrations — automatic

Migrations run **automatically**: the backend and worker each have a `migrate`
initContainer (image `backend/Dockerfile.migrate` = the Atlas CLI + the
`ent/migrate/migrations` directory) that runs `atlas migrate apply` before the
app container starts. It's idempotent and takes a database advisory lock, so the
backend and worker racing on a fresh cluster is safe — one applies, the other
waits and finds nothing pending. On a fresh DB the initContainer retries (pod
restart) until Postgres is reachable, then the app starts. No manual step.

(Temporal's own schema is handled by its `auto-setup` image; only the app DB
goes through this.)

> **Manual fallback**, if you ever need to apply out of band:
>
> ```sh
> kubectl port-forward -n staging pod/postgres-0 5432:5432 &
> atlas migrate apply \
>   --dir "file://backend/ent/migrate/migrations" \
>   --url "postgres://medmarket:<POSTGRES_PASSWORD>@localhost:5432/medmarket?sslmode=disable"
> kill %1
> ```
>
> Keep the Atlas version in `Dockerfile.migrate` aligned with your local
> `atlas version` so the `atlas.sum` integrity check matches.

## 5. Verify

```sh
kubectl get pods -n staging                      # all Ready
kubectl get ingress -n staging                   # note the external IP (3–5 min LB warm-up)
curl http://<INGRESS_IP>/api/health              # {"status":"ok"}
```

- Full journey: run the smoke test against the LB —
  `SMOKE_BASE_URL=http://<INGRESS_IP> task backend:smoke`.
- Traces appear in **Cloud Trace**, logs in **Cloud Logging** (Logs Explorer,
  log name `medmarket`).
- Dashboards (internal): `kubectl port-forward -n staging svc/temporal-ui 8080:8080`
  and `svc/mailpit 8025:8025`.

## 6. Park when idle (budget)

The node pool autoscales to 0 only when nothing is scheduled. To stop paying for
compute while keeping data (PVCs persist):

```sh
kubectl delete -k k8s/overlays/staging    # removes workloads; PVCs + disks remain
```

Re-deploy (step 3) to bring it back — the same persistent disks reattach, so
Postgres/Temporal data survives. To tear down everything including data, delete
the namespace, then `terraform destroy`.
