# infrastructure/kubernetes/

Raw Kubernetes manifests (no Helm) for running the entire Finora fleet —
4 MongoDB instances, NATS JetStream, all 5 Go services, and the frontend —
on a local `kind` cluster. Everything here is "owned by the infra
maintainer" per `architecture/repository-structure.md`, not governed by
`CLAUDE.md`'s app-layer rules, but it stays consistent with the rest of the
repo wherever there's a real choice to make: one Mongo per service, the
gateway as the only JWT validator, the same env var names as
`.env.example`, the same `/live`/`/ready` contract every service already
exposes for `docker-compose.yml`'s healthchecks.

## Prerequisites

- Docker Desktop (or another Docker daemon) running
- [`kind`](https://kind.sigs.k8s.io/#installation-and-usage)
- `kubectl`
- Nothing else — no Helm, no cloud CLI, no registry account. Images are
  built locally and loaded directly into the kind node.

## First-time setup

```bash
# 1. Create the cluster (extraPortMappings for ingress-nginx's 80/443 —
#    see infrastructure/kind-config.yaml's own comment for why)
kind create cluster --name finora --config infrastructure/kind-config.yaml

# 2. Install ingress-nginx's kind-specific manifest (pinned to a known-good
#    release — not vendored into this repo, it's a large third-party
#    project's own file; update the tag deliberately, not by floating on
#    `main`)
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.3/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s

# 3. Point finora.local at your machine (one-time; sudo needed to edit
#    /etc/hosts)
echo "127.0.0.1 finora.local" | sudo tee -a /etc/hosts

# 4. Build all 6 images and load them into the kind node
./infrastructure/kubernetes/scripts/build-and-load.sh

# 5. Apply every manifest in dependency order
./infrastructure/kubernetes/scripts/deploy.sh
```

Then open **http://finora.local** — that's the frontend, Ingress-routed to
the `frontend` Service; `http://finora.local/api/v1/...` is the same
gateway API docker-compose exposes at `localhost:8080`, just reached
through the Ingress instead of a host port mapping.

## After a code change

Rebuild + reload only what changed, then bounce the affected Deployment
(kind's node image cache doesn't auto-invalidate on rebuild — `kubectl
rollout restart` is what actually picks up the freshly-loaded image, since
the tag itself, `:local`, never changes):

```bash
docker build -f services/expense-service/Dockerfile -t finora/expense-service:local .
kind load docker-image finora/expense-service:local --name finora
kubectl -n finora rollout restart deployment/expense-service
```

Or just re-run `scripts/build-and-load.sh` (rebuilds all 6) followed by
`kubectl -n finora rollout restart deployment --all`.

## Layout

```
infrastructure/
  kind-config.yaml           cluster config (ingress port mappings) — used by `kind create cluster`, not `kubectl apply`
  kubernetes/
    00-namespace/              the `finora` namespace everything else lives in
    01-config/                 finora-config ConfigMap (all non-secret env, mirrors .env.example)
                                finora-secrets Secret (JWT signing keys — dev placeholders, same values .env.example ships)
    02-mongo/                  one StatefulSet + headless Service per service's own MongoDB, each with a PVC
    03-nats/                   shared JetStream StatefulSet + headless Service, with a PVC
    04-services/               Deployment + Service per Go service (expense/budget/notification/user-service)
    05-gateway/                gateway Deployment + Service — the only other one with the JWT secret
    06-frontend/               frontend Deployment + Service — no runtime env, NEXT_PUBLIC_API_BASE_URL is baked in at build time
    07-ingress/                single Ingress, path-based routing (see its own comment for why not two subdomains)
    scripts/
      build-and-load.sh        docker build all 6 images, kind load each into the cluster
      deploy.sh                kubectl apply everything in dependency order, with rollout-status gates between stages
```

## Design notes (the decisions worth knowing before changing any of this)

- **StatefulSet, not Deployment+PVC, for Mongo/NATS.** A Deployment's
  RollingUpdate can briefly run two Pods that both try to mount the same
  ReadWriteOnce PVC, which deadlocks. StatefulSet guarantees at most one Pod
  per volume slot even at `replicas: 1` — see `02-mongo/mongo-user.yaml`'s
  comment for the full reasoning.
- **One ConfigMap, `envFrom` on every service**, rather than
  docker-compose's per-service `environment:` subset. Simpler to maintain
  (one file, one source of truth) and harmless — unused keys are just
  ignored by each service's config loader. The one place least-privilege
  *is* enforced is `finora-secrets`: only `user-service` (issues JWTs) and
  `gateway` (verifies them) get `secretRef`'d to it; the other three
  services never see a JWT secret at all, matching CLAUDE.md §2's "the
  gateway is the only JWT validator."
- **Path-based Ingress on one host (`finora.local`), not two subdomains.**
  `/api` → gateway, `/` → frontend. This makes the browser's page origin
  and every API call same-origin, so CORS never actually triggers in normal
  use — `CORS_ALLOWED_ORIGINS` is still set correctly in the ConfigMap as
  defense in depth, but the Ingress shape is what avoids it being a
  recurring local-cluster gotcha.
- **`imagePullPolicy: Never` + local-only `:local` tags, no registry.**
  Appropriate for local kind development; a real cluster would push tagged
  images to a real registry and use `imagePullPolicy: IfNotPresent` (or
  `Always` for a mutable tag) instead.
- **`TRUSTED_PROXIES=10.244.0.0/16`** in the ConfigMap is the one value
  that isn't a straight port of a docker-compose default — docker-compose
  has no reverse proxy in front of the gateway, so it ships empty there.
  Here, the ingress-nginx controller Pod (living in kind's default pod
  CIDR) is a real upstream proxy setting `X-Forwarded-For`, so it has to be
  explicitly trusted — the same configuration a real ingress
  controller/load balancer would need in production, per
  `shared/middleware.RateLimit`'s own doc comment.

## Status

- [x] Namespace, shared ConfigMap/Secret
- [x] All 4 Mongo StatefulSets + PVCs
- [x] NATS StatefulSet + PVC
- [x] All 5 Go services (Deployment + Service, `/live`/`/ready` probes, resource requests/limits)
- [x] Frontend (Deployment + Service)
- [x] Ingress (nginx, path-based, single host)
- [x] Build-and-load + deploy scripts
- [ ] Helm chart — deliberately not yet; per the original scaffolding's own
      plan, raw manifests come first so every object stays readable end to
      end while the underlying concepts are still being learned. Revisit
      once this raw shape has been lived with for a while.
- [ ] Horizontal Pod Autoscaler / multi-replica services — everything here
      is `replicas: 1`, matching docker-compose's own topology. Real
      horizontal scaling (and the StatefulSet vs. stateless-replica
      question that comes with it for Mongo) is future work, not something
      a local single-node kind cluster particularly benefits from anyway.
- [ ] NetworkPolicies — nothing here restricts pod-to-pod traffic within
      the namespace. Fine for a local learning cluster; a real deployment
      would add default-deny + explicit allow rules.

## Tear down

```bash
kind delete cluster --name finora   # also deletes the PVCs' backing hostPath data
```
