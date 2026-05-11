# Agent Runtime Operator

The Agent Runtime Operator is a Kubernetes operator that provides a framework-agnostic way to deploy and manage agentic workloads on Kubernetes. It owns the `Agent`, `AgenticWorkforce`, `AgentGateway`, `AiGateway`, `ToolGateway`, `ToolServer`, `ToolRoute`, `Guard`, and `GuardrailProvider` CRDs that the rest of the Agentic Layer ecosystem builds on.

📖 **Documentation:** https://docs.agentic-layer.ai/agent-runtime-operator/

## Development

### Prerequisites

- **Go** 1.26+
- **Docker**
- **kubectl**
- **kind** (used for local development and E2E tests)
- **make**

### Build and deploy locally

```shell
# Create a local cluster
kind create cluster

# Install Cert Manager for webhook TLS
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cert-manager -n cert-manager --timeout=60s

# Install CRDs into the cluster
make install
# Build docker image
make docker-build
# Load image into kind cluster (not needed if using local registry)
make kind-load
# Deploy the operator to the cluster
make deploy
```

### Test

```shell
make lint       # linting (use `make lint-fix` to auto-fix)
make test       # unit + integration tests, formatter, static analysis
make test-e2e   # E2E tests in a Kind cluster
```

E2E tests keep the Kind cluster after running; clean up with `make cleanup-test-e2e`.

### Verify the local deploy

Apply the bundled CRD samples and inspect the resources the operator reconciles:

```shell
kubectl apply -k config/samples/
kubectl get agents,agenticworkforces,toolservers,toolroutes
```

### Create or Update API and Webhooks

The operator-sdk CLI can be used to create or update APIs and webhooks.
This is the preferred way to add new APIs and webhooks to the operator.
If the operator-sdk CLI is updated, you may need to re-run these commands to update the generated code.

```shell
# Create API for Agent CRD
operator-sdk create api --group runtime --version v1alpha1 --kind Agent

# Create webhook for Agent CRD
operator-sdk create webhook --group runtime --version v1alpha1 --kind Agent --defaulting --programmatic-validation
```

After modifying CRD structs in `api/v1alpha1/*.go`, regenerate manifests with `make manifests && make generate`.

## Contributing

See the [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file).
