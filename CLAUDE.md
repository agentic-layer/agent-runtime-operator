# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Project Overview and Developer Documentation
- @README.md

User Guides and How-To Guides
- @docs/modules/agent-runtime/partials/how-to-guide.adoc
- @docs/modules/agents/partials/how-to-guide.adoc
- @docs/modules/agent-gateways/partials/how-to-guide.adoc

Reference Documentation
- @docs/modules/agent-runtime/partials/reference.adoc
- Overall Agentic Layer Architecture: https://docs.agentic-layer.ai/architecture/main/index.html

Documentation in AsciiDoc format is located in the `docs/` directory.
This folder is hosted as a separate [documentation site](https://docs.agentic-layer.ai/agent-runtime-operator/index.html).

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

- **ToolServer CRD** (`api/v1alpha1/toolserver_types.go`): Defines the ToolServer custom resource for managing tool servers:
  - Protocol specification (mcp for Model Context Protocol)
  - Transport type configuration (stdio, http, sse)
  - Container image and replica configuration (for http/sse)
  - Environment variable configuration
  - Status tracking with conditions and service URL

- **Agent Controller** (`internal/controller/agent_controller.go`): Reconciles Agent resources by:
  - Creating Kubernetes Deployments for agent workloads
  - Managing Services for protocol exposure
  - **Protocol-aware health checking**: Automatically generates appropriate readiness probes
    - A2A agents: HTTP GET with configurable paths (validates agent functionality)
    - OpenAI agents: TCP socket probe (validates service availability)
    - Priority: A2A > OpenAI > No probe
    - No protocols: No readiness probe
  - Handling framework-specific configurations

- **ToolServer Controller** (`internal/controller/toolserver_controller.go`): Reconciles ToolServer resources by:
  - **Transport-aware deployment**:
    - stdio: Marks resource as ready for sidecar injection (no standalone deployment)
    - http/sse: Creates Deployments and Services for standalone tool servers
  - Managing TCP-based health probes for http/sse transports
  - Populating status URL for service discovery
  - Handling environment variable configuration

- **Admission Webhooks** (`internal/webhook/v1alpha1/`): Provides validation and mutation for Agent and ToolServer resources

### Project Structure

```
├── api/                  # CRD definitions and types
├── cmd/main.go           # Operator entry point
├── config/               # Kubernetes manifests and Kustomize configs
│   ├── crd/              # Custom Resource Definitions
│   ├── rbac/             # Role-based access control
│   ├── manager/          # Operator deployment
│   ├── webhook/          # Webhook configurations
│   └── samples/          # Example resources
├── docs/                 # AsciiDoc documentation
├── internal/
│   ├── controller/       # Reconciliation logic
│   └── webhook/          # Admission webhook handlers
└── test/
    ├── e2e/              # End-to-end tests
    └── utils/            # Test utilities
```
