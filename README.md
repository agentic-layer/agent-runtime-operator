# Agent Runtime Operator

The Agent Runtime Operator is a Kubernetes operator that provides a unified, framework-agnostic way to deploy and manage agentic workloads on Kubernetes. It uses Custom Resource Definitions (CRDs) to abstract away framework-specific configurations, simplifying the management of components like wiring of agents with a observability stack or model routers.

The operator is built with the [Operator SDK](https://sdk.operatorframework.io/docs/) framework. Make sure to be familiar with the Operator SDK concepts when working with this project.

## Getting Started

Get started with the Agent Runtime Operator by following our comprehensive guides:

- **[Install the Agent Runtime Operator](https://docs.agentic-layer.ai/agent-runtime-operator/agent-runtime/how-to-guide.html)** - Set up the operator on your Kubernetes cluster
- **[Create your first Agent](https://docs.agentic-layer.ai/agent-runtime-operator/agents/how-to-guide.html)** - Deploy template-based and custom agents

## Development

### Prerequisites

Before contributing to this project, ensure you have the following tools installed:

* **Go**: version 1.24.0 or higher
* **Docker**: version 20.10+ (or a compatible alternative like Podman)
* **kubectl**: The Kubernetes command-line tool
* **kind**: For running Kubernetes locally in Docker
* **make**: The build automation tool
* **Git**: For version control

### Local Environment

Set up your local Kubernetes environment.
You can use any Kubernetes cluster. Kind is used for E2E tests and is used exemplarily here for local development.

```shell
# Create a local Kubernetes cluster (or use an existing one)
kind create cluster
```

### Build and Deploy

Build and deploy the operator locally:

```shell
# Install CRDs into the cluster
make install

# Build docker image
make docker-build

# Load image into kind cluster (not needed if using local registry)
make kind-load

# Deploy the operator to the cluster
make deploy
```

### Unit Tests, code formatting, and static analysis

Run unit tests, code formatting checks, and static analysis whenever you make changes:

```shell
# Run unit tests, code formatter and static analysis
make test
```

### Linting

Several linters are configured via `golangci-lint`. Run the linter to ensure code quality:

```shell
# Run golangci-lint
make lint
````

You can also auto-fix some issues:

```shell
# Auto-fix linting issues
make lint-fix
```

### End-to-End Tests

The E2E tests automatically create an isolated Kind cluster, deploy the operator, run comprehensive tests, and clean up afterward.

```shell
# Run complete E2E test suite
make test-e2e
```

**E2E Test Features:**
- Operator deployment verification
- CRD installation testing
- Webhook functionality testing
- Metrics endpoint validation
- Certificate management verification

**Manual E2E Test Management:**

For faster iteration, you can manually set up and tear down the test cluster and run tests against it.

```shell
# Set up test cluster manually
make setup-test-e2e

# Run tests against existing cluster
KIND_CLUSTER=agent-runtime-operator-test-e2e go test ./test/e2e/ -v -ginkgo.v

# Clean up test cluster
make cleanup-test-e2e
```

**Clean up failed test runs:**

When E2E tests fail, the cleanup step is not run to allow for manual analysis of the failure. As the cluster is in an undefined state, it is recommended to delete the cluster and start fresh:

```shell
make cleanup-test-e2e
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

## Contribution

See [Contribution Guide](https://github.com/agentic-layer/agent-runtime-operator?tab=contributing-ov-file) for details on contribution, and the process for submitting pull requests.
