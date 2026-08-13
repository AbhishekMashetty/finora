#!/usr/bin/env bash
# Applies every manifest in dependency order, waiting for each stage to be
# actually ready before moving on — the same reasoning as docker-compose.yml's
# `depends_on: condition: service_healthy` chain (Mongo/NATS -> the 3 plain
# services -> gateway -> frontend), translated into `kubectl wait`. A flat
# `kubectl apply -f -R infrastructure/kubernetes/` would eventually
# converge too (Kubernetes controllers retry), but pods would visibly
# CrashLoopBackOff a few times first while dependencies race to become
# ready — this avoids that.
#
# Assumes: images already built + loaded (scripts/build-and-load.sh) and
# ingress-nginx already installed (see README.md) — this script only
# applies Finora's own manifests.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE=finora

echo "==> namespace"
kubectl apply -f "$K8S_DIR/00-namespace/"

echo "==> config (ConfigMap + Secret)"
kubectl apply -f "$K8S_DIR/01-config/"

echo "==> Mongo (x4) + NATS"
kubectl apply -f "$K8S_DIR/02-mongo/"
kubectl apply -f "$K8S_DIR/03-nats/"
kubectl -n "$NAMESPACE" rollout status statefulset/mongo-user --timeout=180s
kubectl -n "$NAMESPACE" rollout status statefulset/mongo-expense --timeout=180s
kubectl -n "$NAMESPACE" rollout status statefulset/mongo-budget --timeout=180s
kubectl -n "$NAMESPACE" rollout status statefulset/mongo-notification --timeout=180s
kubectl -n "$NAMESPACE" rollout status statefulset/nats --timeout=180s

echo "==> expense-service, budget-service, notification-service"
kubectl apply -f "$K8S_DIR/04-services/expense-service.yaml"
kubectl apply -f "$K8S_DIR/04-services/budget-service.yaml"
kubectl apply -f "$K8S_DIR/04-services/notification-service.yaml"
kubectl -n "$NAMESPACE" rollout status deployment/expense-service --timeout=120s
kubectl -n "$NAMESPACE" rollout status deployment/budget-service --timeout=120s
kubectl -n "$NAMESPACE" rollout status deployment/notification-service --timeout=120s

echo "==> user-service"
kubectl apply -f "$K8S_DIR/04-services/user-service.yaml"
kubectl -n "$NAMESPACE" rollout status deployment/user-service --timeout=120s

echo "==> gateway"
kubectl apply -f "$K8S_DIR/05-gateway/"
kubectl -n "$NAMESPACE" rollout status deployment/gateway --timeout=120s

echo "==> frontend"
kubectl apply -f "$K8S_DIR/06-frontend/"
kubectl -n "$NAMESPACE" rollout status deployment/frontend --timeout=120s

echo "==> ingress"
kubectl apply -f "$K8S_DIR/07-ingress/"

echo "==> done. If http://finora.local doesn't resolve yet, add this to /etc/hosts:"
echo "      127.0.0.1 finora.local"
