# GitHub Copilot Instructions

This file provides custom instructions for GitHub Copilot when working with the Agent Runtime Operator codebase.

## Project Overview

The Agent Runtime Operator is a Kubernetes operator built with the Operator SDK framework. It provides a unified, framework-agnostic way to deploy and manage agentic workloads on Kubernetes using Custom Resource Definitions (CRDs).

## Tech Stack

- **Language**: Go 1.24.0+
- **Framework**: Operator SDK (controller-runtime v0.22.4)
- **Kubernetes API**: v0.34.3
- **Testing**: Ginkgo v2 and Gomega
- **Tooling**: golangci-lint, kind, kubectl, docker/podman

## Coding Standards

### Go Conventions
- Follow standard Go conventions and idioms
- Use `gofmt` for code formatting (tabs for indentation)
- Use descriptive variable and function names
- Add godoc comments for exported types and functions
- Use kubebuilder markers for CRD generation and RBAC configuration

### Kubernetes Operator Patterns
- Follow controller-runtime best practices
- Use structured logging with `log.FromContext(ctx)`
- Implement proper error handling and reconciliation logic
- Return `ctrl.Result{}` with appropriate requeue logic
- Use `controllerutil.SetControllerReference` for owned resources

### CRD Development
- Define CRDs in `api/v1alpha1/` with appropriate kubebuilder markers
- **Critical**: After modifying CRD structs, always run `make manifests && make generate`
- Use proper JSON tags and validation markers
- Include status subresources with conditions
- Follow Kubernetes API conventions for field names (camelCase in Go, camelCase in JSON)

### File Organization
- Place test files next to implementation files (`*_test.go`)
- Keep controllers in `internal/controller/`
- Keep webhooks in `internal/webhook/v1alpha1/`
- Store sample resources in `config/samples/`

## Architecture Patterns

### Controller Reconciliation
- Implement idempotent reconciliation logic
- Handle resource not found errors gracefully
- Use finalizers for cleanup logic when needed
- Update status conditions to reflect resource state
- Return errors for transient failures (auto-requeue)

### Protocol-Aware Health Checking
When creating Deployments for agents:
- **A2A protocol**: Use HTTP GET readiness probe (validates agent functionality)
- **OpenAI protocol**: Use TCP socket probe (validates service availability)
- **Priority**: A2A > OpenAI > No probe
- No protocols: Skip readiness probe

### Webhook Development
- Implement validation webhooks for CRD validation
- Implement defaulting webhooks to set default values
- Use the `+kubebuilder:webhook` marker for automatic configuration
- Register webhooks in `cmd/main.go`

## Testing Strategy

Follow a three-tier testing approach (prefer simpler tests):

### 1. Unit Tests
- Test isolated logic without external dependencies
- Use standard Go testing (`testing` package)
- Place tests next to implementation files
- Use for: helper functions, data transformations, validation logic

### 2. Integration Tests
- Test controller/webhook logic with Kubernetes API
- Use envtest (provides lightweight Kubernetes API)
- Place tests next to implementation files (`internal/controller/*_test.go`)
- Use for: controller reconciliation, webhook validation, resource creation

### 3. E2E Tests
- Blackbox testing of complete workflows
- Use real Kind cluster with full operator deployment
- Place in `test/e2e/` directory
- Only run as final validation (very slow)
- Use for: complete user workflows, agent deployment, service discovery

### Testing Principles
- Avoid test duplication (don't test unit-tested logic in integration tests)
- Prefer real APIs over extensive mocking (use envtest)
- Make E2E tests blackbox (test behavior, not implementation)

## Common Development Tasks

### Adding a New CRD
```bash
operator-sdk create api --group runtime --version v1alpha1 --kind NewResource --resource --controller
make manifests && make generate
```

### Adding a Webhook
```bash
operator-sdk create webhook --group runtime --version v1alpha1 --kind NewResource --defaulting --programmatic-validation
# Update cmd/main.go to register the webhook
make manifests && make generate
```

### Running Tests and Linters
```bash
make lint          # Run golangci-lint
make test          # Run unit and integration tests
make test-e2e      # Run E2E tests (slow, final validation only)
```

### Building and Deploying
```bash
make install       # Install CRDs
make docker-build  # Build operator image
make kind-load     # Load image into kind cluster
make deploy        # Deploy operator
```

## Important Files and Directories

- `api/v1alpha1/`: CRD type definitions (Agent, AgentGateway, ToolServer, etc.)
- `internal/controller/`: Reconciliation logic for each CRD
- `internal/webhook/v1alpha1/`: Validation and defaulting webhooks
- `config/crd/`: Generated CRD manifests
- `config/samples/`: Example resource YAMLs
- `cmd/main.go`: Operator entry point, webhook registration
- `docs/`: AsciiDoc documentation (hosted separately)
- `test/e2e/`: End-to-end tests

## Best Practices

- Always run `make manifests && make generate` after modifying CRD structs
- Write tests for new functionality (prefer unit/integration over E2E)
- Follow existing patterns for consistency
- Use structured logging with context
- Handle errors appropriately (transient vs permanent)
- Keep reconciliation logic idempotent
- Document complex logic with comments
- Update sample YAMLs when adding new features
