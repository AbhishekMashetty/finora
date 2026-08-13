#!/usr/bin/env bash
# Builds all 6 Finora images locally and loads them straight into the
# `finora` kind cluster's node — no registry involved at all, which is why
# every Deployment in this manifest set sets imagePullPolicy: Never against
# a :local tag (a registry-less tag like this would otherwise make
# Kubernetes try, and fail, to pull from Docker Hub).
#
# Re-run this after any code change before re-deploying — kind's node keeps
# its own separate image cache from your local `docker images`, so a
# `docker build` alone is not enough; the image has to be explicitly
# reloaded every time.
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER_NAME:-finora}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
INGRESS_HOST="${FINORA_INGRESS_HOST:-http://finora.local}"

cd "$REPO_ROOT"

if ! command -v kind >/dev/null 2>&1; then
  echo "error: kind is not installed (https://kind.sigs.k8s.io/#installation-and-usage)" >&2
  exit 1
fi
if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "error: no kind cluster named '$CLUSTER_NAME' — create it first:" >&2
  echo "  kind create cluster --name $CLUSTER_NAME --config infrastructure/kind-config.yaml" >&2
  exit 1
fi

# Go services + gateway: same build context (repo root) and same
# per-service Dockerfile path, matching how docker-compose.yml builds them.
GO_SERVICES=(gateway user-service expense-service budget-service notification-service)

for svc in "${GO_SERVICES[@]}"; do
  echo "==> building finora/${svc}:local"
  docker build -f "services/${svc}/Dockerfile" -t "finora/${svc}:local" .
  echo "==> loading finora/${svc}:local into kind cluster '${CLUSTER_NAME}'"
  kind load docker-image "finora/${svc}:local" --name "$CLUSTER_NAME"
done

echo "==> building finora/frontend:local (NEXT_PUBLIC_API_BASE_URL=${INGRESS_HOST})"
docker build \
  -f frontend/Dockerfile \
  --build-arg "NEXT_PUBLIC_API_BASE_URL=${INGRESS_HOST}" \
  -t finora/frontend:local \
  frontend
echo "==> loading finora/frontend:local into kind cluster '${CLUSTER_NAME}'"
kind load docker-image finora/frontend:local --name "$CLUSTER_NAME"

echo "==> all 6 images built and loaded"
