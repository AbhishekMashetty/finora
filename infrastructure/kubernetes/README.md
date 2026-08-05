# infrastructure/kubernetes/

Raw Kubernetes manifests for running Finora on a local `kind` cluster, built
up incrementally as a learning exercise (see `architecture/repository-structure.md`
— this directory is explicitly "owned by the infra maintainer," not governed
by the same rules as the Go/frontend app layers in `CLAUDE.md`).

No Helm yet — deliberately raw `kubectl apply -f` manifests so every object
is readable end to end while the underlying concepts are still being
learned. Helm comes later, once the raw shape is well understood, as a
templating layer over what's already here.

## Local cluster

```bash
kind create cluster --name finora
kubectl config set-context --current --namespace=finora   # done once already
```

## Layout

```
namespace/
  namespace.yaml        # the `finora` namespace everything else lives in
mongo-user/
  deployment.yaml        # mongo-user's own MongoDB — no PVC yet (Module 3),
  service.yaml            # data is lost on pod restart until then
```

Apply in order: `namespace/` first, then each service's own folder.

```bash
kubectl apply -f infrastructure/kubernetes/namespace/
kubectl apply -f infrastructure/kubernetes/mongo-user/
```

## Status

- [x] `mongo-user` (Deployment + Service, ephemeral storage)
- [ ] `user-service` itself (ConfigMap/Secret, probes, own image loaded into kind)
- [ ] PersistentVolumeClaim for `mongo-user` (durable storage)
- [ ] Remaining services (expense/budget/notification-service) + NATS
- [ ] Ingress for gateway + frontend
- [ ] Helm chart
