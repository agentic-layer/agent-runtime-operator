# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
For more details, refer to the [README.md](README.md) file.

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
  - Framework specification (google-adk, custom)
  - Container image and replica configuration
  - Protocol definitions (A2A, OpenAI)
  - Status tracking with conditions

- **AgentGateway CRD** (`api/v1alpha1/agentgateway_types.go`): Defines the AgentGateway custom resource for exposing agents via a unified gateway:
  - Gateway provider abstraction (KrakenD, Envoy, Nginx)
  - Routing strategies (path-based, subdomain-based)
  - IAP (Identity-Aware Proxy) integration for security
  - TLS configuration and certificate management
  - Agent reference and selective exposure controls

- **Agent Controller** (`internal/controller/agent_controller.go`): Reconciles Agent resources by:
  - Creating Kubernetes Deployments for agent workloads
  - Managing Services for protocol exposure
  - **Protocol-aware health checking**: Automatically generates appropriate readiness probes
    - A2A agents: HTTP GET `/a2a/.well-known/agent-card.json` (validates agent functionality)
    - OpenAI agents: TCP socket probe (validates service availability)
    - No protocols: No readiness probe
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
│   ├── controller/       # Reconciliation logic and health check implementation
│   └── webhook/          # Admission webhook handlers
├── docs/
│   └── examples/        # Health check implementation examples
└── test/
    ├── e2e/             # End-to-end tests
    └── utils/           # Test utilities
```

### Resource Examples

**Using Agent Templates (recommended for framework-agnostic agents):**
```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: weather-agent-template
spec:
  framework: google-adk
  description: "A helpful weather information agent"
  instruction: "You are a weather agent that provides current weather information and forecasts."
  model: "gemini/gemini-2.5-flash"
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

**Using Custom Docker Images:**
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
  env:
    - name: PORT
      value: "8080"
  envFrom:
    - secretRef:
        name: api-key-secret
```

#### AgentGateway Resource
```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgentGateway
metadata:
  name: main-gateway
spec:
  agentGatewayClassName: "krakend"
  replicas: 2
  timeout: "60s"
```

## Testing

- Unit tests use Ginkgo/Gomega testing framework
- E2E tests automatically create/teardown Kind clusters
- Tests cover controller reconciliation and webhook validation
- Use `make test` for unit tests, `make test-e2e` for end-to-end tests

## Health Check Architecture

The operator implements **protocol-aware health checking** that automatically generates appropriate Kubernetes readiness probes based on agent protocol specifications:

### Protocol-Based Health Checks

**A2A Agents (Agent-to-Agent Protocol):**
- **Probe Type**: HTTP GET request
- **Endpoint**: `/a2a/.well-known/agent-card.json`
- **Port**: 8000 (default)
- **Purpose**: Validates that A2A framework is initialized and agent skills are loaded
- **Benefits**: Meaningful readiness signal that confirms agent functionality, not just HTTP server availability

**OpenAI Compatible Agents:**
- **Probe Type**: TCP socket connection
- **Port**: 8000 (or specified protocol port)
- **Purpose**: Validates service is listening and accepting connections
- **Benefits**: Lightweight health check suitable for stateless API endpoints

**Agents without Recognized Protocols:**
- **Probe Type**: None (no readiness probe generated)
- **Purpose**: Allows custom health checking or agents that don't need readiness validation

### Implementation Details

The health check logic is implemented in `internal/controller/agent_controller.go`:

- `generateReadinessProbe()`: Creates appropriate probe configuration based on agent protocols
- `hasA2AProtocol()`: Detects A2A protocol in agent specification
- `hasOpenAIProtocol()`: Detects OpenAI protocol in agent specification
- `probesEqual()`: Compares probe configurations to determine if deployment updates are needed

### Migration from Legacy Health Endpoints

**Previous Approach (Deprecated):**
- Generic `/health` endpoints in each agent
- Only validated HTTP server availability
- Required manual implementation in every agent

**Current Approach (Recommended):**
- Operator-managed, protocol-aware health checking
- Validates actual agent functionality for A2A agents
- Zero health code required in agents
- Centralized management and consistent behavior

## Configuration

- Uses golangci-lint for code quality (config in `.golangci.yml`)
- Supports secure metrics endpoint with TLS
- Webhook certificates managed via cert-manager
- Leader election enabled for high availability
