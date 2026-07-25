# Deployment Notes & Gotchas

Hard-won lessons from the first GKE deployment. Organized so you can Ctrl-F the
**symptom / error string** you're seeing. Each entry: what you see → why → fix.

Context: GKE Standard, zonal cluster `medmarket` in `us-central1-a`, VPC-native,
Workload Identity, Kustomize manifests, Artifact Registry images, terraform infra.

---

## Cluster & node sizing

### Pods stuck `Pending`, `0/N nodes are available: N Insufficient cpu`
- **Cause:** `e2-medium` is a **shared-core** machine type — GKE reports only
  **940m allocatable CPU** (verify: `kubectl describe node <node>` → `Allocatable:
  cpu:`). GKE's per-node system DaemonSets (Calico network-policy, the Workload
  Identity metadata server, logging/monitoring) consume most of it, leaving almost
  nothing for the app. Adding more small nodes doesn't help — each new node just
  pays the same fixed system tax.
- **Fix:** use a **dedicated-vCPU** machine type. `e2-standard-2` = 2 real vCPU →
  **~1930m allocatable**; the whole app (~850m CPU) fits on one node.
  `machine_type` changes apply **in-place** (rolling node replacement) in the
  google provider `~> 7.0` — a `~ update`, not destroy+recreate.
- **Also:** the autoscaler won't scale past `max_node_count` — the describe event
  `NotTriggerScaleUp: ... max node group size reached` means raise the max, but if
  even the max can't fit, it's a per-node sizing problem (above), not a count one.

### `ZONE_RESOURCE_POOL_EXHAUSTED` creating a node
- **Cause:** transient — the zone temporarily lacks capacity for that machine type.
  Not a config problem.
- **Fix:** retry `terraform apply` in a few minutes. If it persists, try a
  different machine family in the **same zone** (e.g. `n2-standard-2` — separate
  capacity pool from E2) to stay in-region. Changing zone means recreating the
  (zonal) cluster — last resort.

---

## Node permissions (image pulls)

### Private images `ImagePullBackOff`; public images (e.g. mailpit) pull fine
- **Cause:** image pulls happen at the **node/kubelet** level using the **node
  service account** — this is *separate* from Workload Identity (which is
  pod-level). The node pool had no `service_account` set, so it ran as the default
  Compute Engine SA, which on newer projects no longer gets Editor automatically →
  no Artifact Registry read.
- **Fix:** grant the node SA the roles it needs (in `gke.tf`):
  `roles/container.defaultNodeServiceAccount` + `roles/artifactregistry.reader`,
  using `data.google_compute_default_service_account`.
- **Note:** the GKE console recommendation *"Grant
  roles/container.defaultNodeServiceAccount..."* **lags** — it stays flagged (even
  "high priority") for hours after you've fixed it. Proof it's actually fixed:
  private images pull.

---

## Migrations

### Migrate initContainer image tag not found: `arigaio/atlas:0.36.1: not found`
- **Cause:** wrong tag. Atlas versioning is the **`1.2.x`** line, not `0.36.x`.
- **Fix:** pin a real published tag — `arigaio/atlas:1.2.3`. Keep it near your
  local `atlas version`. Verify before deploy: `docker build` the image, then
  `docker run <img> version` and `docker run <img> migrate validate --dir
  file:///migrations`.

### Migrate initContainer `Init:Error`/`CrashLoopBackOff` with **no logs**; describe shows `exec: "atlas": executable file not found in $PATH`
- **Cause:** the atlas image is minimal (no shell) with `atlas` as its
  **ENTRYPOINT**. The manifest used `command: ["atlas", ...]`, which *overrides*
  the entrypoint, and a bare `atlas` doesn't resolve on `$PATH`. No logs = the
  container never started (it's not an atlas error, it's an exec error).
- **Fix:** use **`args:`** not `command:` — `args: ["migrate", "apply", ...]`
  appends to the image's built-in `atlas` entrypoint.
- **Lesson:** verify the manifest's *actual invocation pattern*. `docker run <img>
  <args>` tests entrypoint+args; it does **not** test the command-override the
  manifest was using.

---

## Secrets & config

### atlas / DB connection fails: `sql/sqlclient: parse open url: invalid port ...`
- **Cause:** `POSTGRES_PASSWORD` was generated with `openssl rand -base64`, which
  emits URL-unsafe characters (`+`, `/`, `=`). Embedded unescaped in
  `postgres://user:PASSWORD@host:5432/...`, they break URL parsing.
- **Fix:** generate a **URL-safe** password: `openssl rand -hex 32`. Update **both**
  `POSTGRES_PASSWORD` and `DATABASE_URL` (they must match).
- **Gotcha:** Postgres only reads `POSTGRES_PASSWORD` on **first init** (empty data
  dir). Changing it later requires wiping the PVC — `kubectl delete namespace
  staging` on a fresh env (no data to lose), or `ALTER USER` in the DB.

---

## App-specific

### `temporal-ui` `CrashLoopBackOff`: `config file corrupted ... cannot unmarshal !!str 'tcp://...' into int`
- **Cause:** Kubernetes auto-injects legacy Docker-link Service env vars. A Service
  named `temporal-ui` produces `TEMPORAL_UI_PORT=tcp://<ip>:8080`, which collides
  with the app's own `TEMPORAL_UI_PORT` config (expects an int port).
- **Fix:** set it explicitly in the Deployment env — `TEMPORAL_UI_PORT: "8080"`
  (an explicit env var overrides the injected one). General cure for this class:
  `enableServiceLinks: false` on the pod spec.

---

## Ingress / load balancer

> The Ingress was the longest fight — **four** compounding issues. Work them in
> this order; each unblocks the next symptom.

### Ingress has no `ADDRESS`, no Events, no Annotations after many minutes
- **Cause:** the **HTTP Load Balancing addon was disabled** on the cluster
  (terraform created it without an `addons_config` block). With it off, GKE's L7
  controller (`glbc`) doesn't run, so `gce` Ingresses are never processed.
  Verify: `gcloud container clusters describe medmarket --zone us-central1-a
  --format="yaml(addonsConfig)"` — if `httpLoadBalancing` is **absent**, it's off.
- **Fix:** `gcloud container clusters update medmarket --zone us-central1-a
  --update-addons=HttpLoadBalancing=ENABLED`.
- **IaC note:** capturing this in terraform needs an `addons_config` block that
  *also* re-declares the other enabled addons (esp. `gce_persistent_disk_csi_driver`
  — your PVCs depend on it), or terraform will disable them.

### `kubectl get ingressclass` is empty; Ingress with `ingressClassName: gce` is ignored
- **Cause:** **GKE requires the annotation `kubernetes.io/ingress.class: "gce"`,
  NOT `spec.ingressClassName`.** GKE deliberately creates no `gce` IngressClass
  object, so an empty `get ingressclass` is normal, and `ingressClassName: gce`
  never matches anything.
- **Fix:** use the annotation form:
  ```yaml
  metadata:
    annotations:
      kubernetes.io/ingress.class: "gce"
  ```
  The `kubernetes.io/ingress.class is deprecated` warning on apply is a *generic
  Kubernetes* warning and harmless — GKE requires it anyway.
  (Ref: [GKE Ingress docs](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/load-balance-ingress))

### Ingress event `Translation failed: ... service "..." is type "ClusterIP", expected "NodePort" or "LoadBalancer" when not using NEGs`
- **Cause:** for container-native load balancing, each backing Service needs the
  **NEG annotation** — it is **not** auto-added.
- **Fix:** add to every Service the Ingress routes to:
  ```yaml
  metadata:
    annotations:
      cloud.google.com/neg: '{"ingress": true}'
  ```
  Live-fix without a redeploy: `kubectl annotate service <name> -n staging
  'cloud.google.com/neg={"ingress": true}' --overwrite`. **Do not** raw
  `kubectl apply -f backend.yaml` to fix a Service — that file also carries the
  Deployment, and a non-kustomize apply resets its hashed Secret reference.

### `curl` to the LB returns `(52) Empty reply from server` right after the address appears
- **Cause:** the IP is assigned before the backends finish registering and passing
  their first health checks. No healthy backend → empty reply.
- **Fix:** wait a few minutes. Watch the health state:
  `kubectl describe ingress medmarket -n staging | grep -A1
  ingress.kubernetes.io/backends` — it flips `Unknown` → `HEALTHY`. The LB health
  check derives from the pod's **readiness probe** (`/api/health`), so make sure
  that probe is correct.

---

## Operational / workflow lessons

- **`deploy-staging.yml` does not run terraform.** Infra changes (nodes, IAM,
  addons) need a manual `terraform apply`. Fixing a stuck cluster ≠ needing a
  redeploy — the images are already in Artifact Registry and the manifests are
  already applied.
- **Parking a GKE cluster to $0 with an autoscaling pool is genuinely fiddly.**
  Two things fight you: (1) scaling workloads to 0 does **not** drain the nodes —
  GKE's autoscaler keeps a floor to run its own kube-system pods (`kube-dns` runs
  2 replicas with anti-affinity → needs 2 nodes); (2) a plain
  `resize --num-nodes 0` **bounces back** — the autoscaler re-adds nodes for those
  same kube-system pods. The only reliable park-to-$0: **disable autoscaling,
  then resize to 0** (now it holds). Also **delete the Ingress** — the L7 LB bills
  separately from nodes (~$18/mo) and stays up otherwise. `task cluster-down` does
  all three (delete ingress → disable autoscaling → resize 0); `task cluster-up`
  re-enables autoscaling → `apply -k` (autoscaler then scales nodes up for the
  workloads). Caveat: disabling autoscaling via gcloud creates terraform drift —
  don't `terraform apply` while parked, it'd re-enable autoscaling and wake nodes.
- **Merge to `main` without deploying:** put `[skip ci]` in the **merge commit
  message** (not the PR title/description).
- **`git stash` does not stash untracked files** by default — new files stay in
  the working tree and can be left behind across a branch switch. `go.work.sum`
  churns across branches (Go tooling rewrites it); `git restore go.work.sum` to
  discard the churn.
- **Live-debug technique:** `kubectl patch` / `kubectl annotate` apply a fix to the
  running object instantly (no image rebuild) to validate it — then fix the source
  manifest so it's permanent.
- **First-deploy rollout-wait timeouts are not necessarily failures** — a cold
  cluster (node provisioning + image pulls + the Postgres→migrate→backend chain)
  can exceed the workflow's `rollout status --timeout`. Check `kubectl get pods`
  directly before assuming it's broken.
