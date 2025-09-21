# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Kubernetes operator for the agentic layer agent runtime component, built using the Kubebuilder framework. The operator manages Agent custom resources that abstract framework-specific agent deployment and configuration.

## Development Commands

### Building and Testing
```bash
# Build the operator
make build

# Run tests (unit tests excluding e2e)
make test

# Run e2e tests with isolated Kind cluster
make test-e2e

# Run linting
make lint

# Fix linting issues automatically
make lint-fix

# Format code
make fmt

# Run go vet
make vet
```

### Local Development
```bash
# Generate CRDs, RBAC, and webhooks
make manifests

# Generate deep copy methods
make generate

# Run controller locally against current cluster
make run

# Install CRDs into cluster
make install

# Deploy operator to cluster
make deploy
```

### Container Operations
```bash
# Build Docker image
make docker-build

# Load image into Kind cluster
make kind-load

# Push image to registry
make docker-push
```

## Architecture

### Core Components

- **Agent CRD** (`api/v1alpha1/agent_types.go`): Defines the Agent custom resource with:
  - Framework specification (google-adk, flokk, autogen)
  - Container image and replica configuration
  - Protocol definitions (A2A, OpenAI)
  - Status tracking with conditions

- **Agent Controller** (`internal/controller/agent_controller.go`): Reconciles Agent resources by:
  - Creating Kubernetes Deployments for agent workloads
  - Managing Services for protocol exposure
  - Handling framework-specific configurations

- **Admission Webhooks** (`internal/webhook/v1alpha1/`): Provides validation and mutation for Agent resources

### Project Structure

```
├── api/v1alpha1/           # CRD definitions and types
├── cmd/main.go            # Operator entry point
├── config/                # Kubernetes manifests and Kustomize configs
│   ├── crd/              # Custom Resource Definitions
│   ├── rbac/             # Role-based access control
│   ├── manager/          # Operator deployment
│   ├── webhook/          # Webhook configurations
│   └── samples/          # Example Agent resources
├── internal/
│   ├── controller/       # Reconciliation logic
│   └── webhook/          # Admission webhook handlers
└── test/
    ├── e2e/             # End-to-end tests
    └── utils/           # Test utilities
```

### Agent Resource Example

```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: weather-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/weather-agent:0.3.0
  protocols:
    - type: A2A
  replicas: 1
```

## Testing

- Unit tests use Ginkgo/Gomega testing framework
- E2E tests automatically create/teardown Kind clusters
- Tests cover controller reconciliation and webhook validation
- Use `make test` for unit tests, `make test-e2e` for end-to-end tests

## Configuration

- Uses golangci-lint for code quality (config in `.golangci.yml`)
- Supports secure metrics endpoint with TLS
- Webhook certificates managed via cert-manager
- Leader election enabled for high availability