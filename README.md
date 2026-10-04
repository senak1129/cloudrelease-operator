# CloudRelease Operator

[中文](README_CN.md)

A Kubernetes operator that manages simple web application deployments through a custom resource.
It translates a high-level `CloudRelease` declaration into standard `Deployment` and `Service` resources.

## What it does

Instead of writing full Deployment/Service YAML, users submit a `CloudRelease`:

```yaml
apiVersion: delivery.cloudrelease.dev/v1alpha1
kind: CloudRelease
metadata:
  name: demo
spec:
  image: nginx:1.27-alpine
  replicas: 3
  port: 80
```

The operator automatically creates and manages:

- A `Deployment` with the specified image, replicas, and container port
- A `Service` exposing the port
- Status tracking (`readyReplicas`, `observedGeneration`)

It also handles:

- **Self-healing**: deleted Services or Deployments are recreated
- **Scale up/down**: changing `spec.replicas` updates the Deployment
- **Cascading deletion**: deleting the CR removes all child resources
- **Leader election**: multi-replica deployments with a single active reconciler

## Quick start (local development)

### Prerequisites

- Go 1.24+
- kubectl
- A local Kubernetes cluster (kind, minikube, or Docker Desktop)

### Install CRD

```sh
kubectl apply -f config/crd/bases/
```

### Run the operator locally

```sh
go run ./cmd/main.go
```

### Create a CloudRelease

```sh
kubectl apply -f config/samples/delivery_v1alpha1_cloudrelease.yaml
```

Verify:

```sh
kubectl get cloudrelease
kubectl get deployment
kubectl get service
kubectl get pods
```

### Clean up

```sh
kubectl delete -f config/samples/delivery_v1alpha1_cloudrelease.yaml
```

## Testing

Three test layers:

```sh
# Unit tests (fake client, fast)
go test ./internal/controller/

# EnvTest (real kube-apiserver, verifies CRD schema)
export KUBEBUILDER_ASSETS=$PWD/bin/k8s/1.36.2-linux-amd64
go test -tags=envtest ./test/envtest/

# E2E script (kind cluster, full flow)
./hack/e2e.sh
```

## Architecture

```
CloudRelease (CR)
  spec.image / replicas / port
        │
        ▼
  Reconcile loop
        │
        ├──▶ Deployment (apps/v1)
        │      replicas = spec.replicas
        │      image    = spec.image
        │      port     = spec.port
        │
        ├──▶ Service (v1)
        │      port = spec.port
        │
        └──▶ Status
               observedGeneration
               readyReplicas
```

Each reconciliation:

1. Reads the `CloudRelease` object
2. Compares desired state (spec) with actual state (Deployment/Service)
3. Creates or patches child resources to match
4. Writes back status

The loop is **level-triggered**: it always reads current state and converges,
regardless of what events happened. Restarting the operator requires no manual
recovery.

## Design decisions

- **CreateOrPatch for idempotency**: every child resource is reconciled with
  `controllerutil.CreateOrPatch`, so running Reconcile multiple times produces
  the same result.
- **UID-based selector**: Deployments use the CloudRelease UID as a label
  selector, preventing the operator from adopting unrelated resources.
- **Owner references**: child resources set `OwnerReference` to the parent CR,
  enabling automatic cascading deletion.
- **No webhook**: validation is handled by CRD schema markers
  (`+kubebuilder:validation`), not an admission webhook. Keeps the project
  simple.
- **Status subresource**: status is written through `/status`, isolated from
  spec updates.

## Project structure

```
api/v1alpha1/       # CloudRelease type definition and CRD markers
internal/controller/ # Reconcile logic and unit tests
cmd/main.go         # Manager entrypoint
config/
  crd/bases/        # Generated CRD YAML
  rbac/             # Generated RBAC roles
  samples/          # Example CloudRelease
test/envtest/       # EnvTest integration tests
hack/e2e.sh         # E2E script
```

## License

Apache License 2.0.
