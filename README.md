# Agent Runtime Operator

The Agent Runtime Operator is a Kubernetes operator built with the [Operator SDK](https://sdk.operatorframework.io/) framework. Its purpose is to provide a unified, framework-agnostic way to deploy and manage agentic workloads on Kubernetes. It uses Custom Resource Definitions (CRDs) to abstract away framework-specific configurations, simplifying the management of components like wiring of agents with a observability stack or model routers.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [End-to-End (E2E) Testing](#end-to-end-e2e-testing)
- [Testing Tools and Configuration](#testing-tools-and-configuration)
- [Sample Data](#sample-data)
- [Contributing](#contributing)

----
## Prerequisites

Before working with this project, ensure you have the following tools installed on your system:

  * **Go**: version 1.24.0 or higher
  * **Docker**: version 20.10+ (or a compatible alternative like Podman)
  * **kubectl**: The Kubernetes command-line tool
  * **kind**: For running Kubernetes locally in Docker
  * **make**: The build automation tool
  * **Git**: For version control

----

## Getting Started

Follow these steps to get the operator up and running on a local Kubernetes cluster.

### Prerequisites
```shell
# Create a local Kubernetes cluster using kind
kind create cluster
```

```bash
# Install cert-manager for webhook support (update the version to the latest stable if needed)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml
# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=cert-manager -n cert-manager --timeout=60s
```

### Installation
```bash
# Install the Agent Runtime Operator (update the version to the latest stable if needed)
kubectl apply -f https://github.com/agentic-layer/agent-runtime-operator/releases/download/v0.2.3/install.yaml
# Wait for the operator to be ready
kubectl wait --for=condition=Available --timeout=60s -n agent-runtime-operator-system deployment/agent-runtime-operator-controller-manager
```

## Development

Follow the prerequisites above to set up your local environment.
Then follow these steps to build and deploy the operator locally:

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

## Configuration

### Environment Variables

The operator can be configured using the following environment variables:

- `ENABLE_WEBHOOKS` - Set to `false` to disable admission webhooks (default: `true`)
- `METRICS_BIND_ADDRESS` - Address for metrics server (default: `:8443`)
- `HEALTH_PROBE_BIND_ADDRESS` - Address for health probes (default: `:8081`)

### Custom Resource Configuration

To deploy an agent, you define an `Agent` resource. Here are example configurations:

**Using a Custom Docker Image:**
```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  labels:
    app.kubernetes.io/name: agent-runtime-operator
    app.kubernetes.io/managed-by: kustomize
  name: weather-agent
spec:
  framework: google-adk  # Supported: google-adk, flokk, autogen
  image: ghcr.io/agentic-layer/weather-agent:0.3.0
  protocols:
    - type: A2A  # Agent-to-Agent protocol
  replicas: 1  # Number of agent replicas (optional, default: 1)
  env:
    - name: PORT
      value: "8080"
  envFrom:
    - secretRef:
        name: api-key-secret
```

**Using Agent Templates (no custom image needed):**
```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  labels:
    app.kubernetes.io/name: agent-runtime-operator
    app.kubernetes.io/managed-by: kustomize
  name: weather-agent-template
spec:
  framework: google-adk  # Currently only google-adk supports templates
  description: "A helpful weather information agent"
  instruction: "You are a weather agent that provides current weather information and forecasts."
  model: "gemini/gemini-2.0-flash"
  subAgents:
    - name: forecast_agent
      url: "https://example.com/forecast-agent.json"
    - name: location_agent
      url: "https://example.com/location-agent.json"
  tools:
    - name: weather_api
      url: "https://weather.mcpservers.org/mcp"
    - name: web_fetch
      url: "https://remote.mcpservers.org/fetch/mcp"
  protocols:
    - type: A2A
  replicas: 1
  envFrom:
    - secretRef:
        name: api-key-secret
```


## End-to-End (E2E) Testing

### Prerequisites for E2E Tests

- **kind** must be installed and available in PATH
- **Docker** running and accessible
- **kubectl** configured and working

### Running E2E Tests

The E2E tests automatically create an isolated Kind cluster, deploy the operator, run comprehensive tests, and clean up afterwards.

```bash
# Run complete E2E test suite
make test-e2e
```

The E2E test suite includes:
- Operator deployment verification
- CRD installation testing
- Webhook functionality testing
- Metrics endpoint validation
- Certificate management verification

### Manual E2E Test Setup

If you need to run E2E tests manually or inspect the test environment:

```bash
# Set up test cluster (will create 'agent-runtime-operator-test-e2e' cluster)
make setup-test-e2e
```
```bash
# Run E2E tests against the existing cluster
KIND_CLUSTER=agent-runtime-operator-test-e2e go test ./test/e2e/ -v -ginkgo.v
```
```bash
# Clean up test cluster when done
make cleanup-test-e2e
```

## Testing Tools and Configuration

## Sample Data

The project includes sample `Agent` custom resources to help you get started.

  * **Where to find sample data?**
    Sample manifests are located in the `config/samples/` directory.

  * **How to deploy a sample agent?**
    You can deploy the sample "weather-agent" with the following `kubectl` command:

    ```bash
    kubectl apply -k config/samples/
    ```

  * **How to verify the sample agent?**
    After applying the sample, you can check the status of the created resources:

    ```bash
    # Check the agent's status
    kubectl get agents weather-agent -o yaml
    ```
    ```bash
    # Check the deployment created by the operator
    kubectl get deployments -l app.kubernetes.io/name=weather-agent
    ```
## Contributing

We welcome contributions to the Agent Runtime Operator! Please follow these guidelines:

### Setup for Contributors

1. **Fork and clone the repository**
2. **Install pre-commit hooks** (mandatory for all contributors):
   ```bash
   brew bundle
   ```
   ```bash
   # Install hooks for this repository
   pre-commit install
   ```

3. **Verify your development environment**:
   ```bash
   # Run all checks that pre-commit will run
   make fmt vet lint test
   ```

### Code Style and Standards

- **Go Style**: We follow standard Go conventions and use `gofmt` for formatting
- **Linting**: Code must pass golangci-lint checks (see `.golangci.yml`)
- **Testing**: All new features must include appropriate unit tests
- **Documentation**: Update relevant documentation for new features

### Development Workflow

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/PAAL-1234-your-feature-name
   ```

2. **Make your changes** following the code style guidelines

3. **Run development checks**:
   ```bash
   # Format code
   make fmt

   # Run static analysis
   make vet

   # Run linting
   make lint

   # Run unit tests
   make test

   # Generate updated manifests if needed
   make manifests generate
   ```

4. **Test your changes**:
   ```bash
   # Run E2E tests to ensure everything works
   make test-e2e
   ```
5. **Update Documentation**:
   Documentation is located in the [`/docs`](/docs) directory. We use the **[Diátaxis framework](https://diataxis.fr/)** for structure and **Antora** to build the site. Please adhere to these conventions when making updates.

6. **Commit your changes** with a descriptive commit message

7**Submit a pull request** with:
   - Clear description of the changes
   - Reference to any related issues
   - Screenshots/logs if applicable

Thank you for contributing to the Agent Runtime Operator!
