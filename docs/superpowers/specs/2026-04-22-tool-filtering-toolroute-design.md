# Tool Filtering via ToolRoute

**Date:** 2026-04-22
**Status:** Draft

## Goal

Let tool-server owners expose only a subset of an MCP server's tools, enforced at the gateway, and let different agents see different subsets of the same physical server without duplicating the server.

This is achieved by introducing a new namespace-scoped CR, `ToolRoute`, which represents a named, filtered exposure of an upstream MCP server through a `ToolGateway`. `AgentTool` attaches tools exclusively via `toolRouteRef`. `ToolServer` goes back to being a pure deployment concern.

## Non-Goals

The following are intentionally deferred to future work, to keep the first iteration focused:

- **Global "ceiling" filter on `ToolServer`.** Server-wide hard cap on what can ever be exposed, irrespective of any route. Can be added later as `ToolServer.spec.toolFilter`.
- **Consumer-identity enforcement on routes.** Any agent that knows a route's URL can use it. Useful for defense-in-depth; orthogonal to the filtering mechanism.
- **Active upstream discovery.** Gateway does not proactively call `tools/list` on upstream to validate filter spec or populate observed tool lists. Enforcement is passive.
- **Rich filter expressions.** No annotation-based filtering (`readOnlyHint`), no regex, no CEL. Names + glob patterns only.
- **Per-route guardrails.** Guardrails stay on `ToolGateway`. Routes do not override or add guardrails in this iteration.
- **Static / secret-injected headers on upstream.** Only headers propagated from the inbound A2A request are forwarded. No `staticHeaders` with Secret refs on `ToolRoute.upstream.external`.

## Architecture Overview

### Resource Relationships

```
ToolServer (deployment)   <------ ToolRoute (routing+filter) ------> ToolGateway (impl target)
                                        ^
                                        | toolRouteRef
                                        |
                                    AgentTool
```

- `ToolServer` describes *what runs*. It no longer knows about gateways.
- `ToolRoute` describes *how one upstream is exposed*: which gateway, which filter, where to send traffic.
- `Agent` consumes tools exclusively through `toolRouteRef`.

### Request Flow

```
agent-pod
  |-- reads AGENT_TOOLS env (URL = ToolRoute.status.url)
  |-- MCP client forwards headers per AgentTool.propagatedHeaders
  v
tool-gateway pod (managed by tool-gateway-agentgateway / tool-gateway-envoy-ai-gateway)
  |-- receives request at the URL set in ToolRoute.status.url
  |-- applies toolFilter:
  |     - rewrites tools/list to drop filtered names
  |     - rejects tools/call for filtered names
  |-- proxies all headers transparently
  v
upstream
  |-- cluster ToolServer via its Service  (upstream.toolServerRef)
  '-- external URL                         (upstream.external.url)
```

Enforcement is **passive**: the gateway implementation applies the filter at request time. The operator does not compile the glob spec, does not call upstream, and does not populate an observed tool list.

### Design Principles

- **Single attachment path for agents.** Every tool an agent uses is a `ToolRoute`. Removes the silent bypass where a bare `AgentTool.url` today skips the gateway.
- **Routes are auditable.** `kubectl get toolroute -A` enumerates every way every server is exposed, with its filter and URL. No grep across Agents.
- **Filter translation lives in the gateway implementation.** The CRD stores the spec verbatim. Each tool-gateway operator translates globs into its own filter primitives.
- **Breaking changes are acceptable.** We are in `v1alpha1`; cleaner API is preferred over a compatibility shim.

## CRD Changes

### New CRD: `ToolRoute`

Namespace-scoped, group `runtime.agentic-layer.ai/v1alpha1`.

```go
type ToolRouteSpec struct {
    // ToolGatewayRef identifies the ToolGateway hosting this route.
    // Namespace defaults to the ToolRoute's namespace.
    // +kubebuilder:validation:Required
    ToolGatewayRef corev1.ObjectReference `json:"toolGatewayRef"`

    // Upstream specifies the MCP server this route proxies.
    // Exactly one of ToolServerRef or External must be set (enforced by CEL validation).
    // +kubebuilder:validation:Required
    Upstream ToolRouteUpstream `json:"upstream"`

    // ToolFilter restricts which tools are exposed through this route.
    // If nil, all tools pass through unfiltered.
    // +optional
    ToolFilter *ToolFilter `json:"toolFilter,omitempty"`
}

type ToolRouteUpstream struct {
    // ToolServerRef references a cluster-local ToolServer.
    // Namespace defaults to the ToolRoute's namespace.
    // Mutually exclusive with External.
    // +optional
    ToolServerRef *corev1.ObjectReference `json:"toolServerRef,omitempty"`

    // External describes a remote MCP server reachable at an HTTP URL.
    // Mutually exclusive with ToolServerRef.
    // Nested struct leaves room for future fields (auth, TLS, timeouts) without breaking the schema.
    // +optional
    External *ExternalUpstream `json:"external,omitempty"`
}

type ExternalUpstream struct {
    // Url is the HTTP/HTTPS endpoint of the external MCP server.
    // +kubebuilder:validation:Format=uri
    // +kubebuilder:validation:Required
    Url string `json:"url"`
}

type ToolFilter struct {
    // Allow is an allowlist of tool names and glob patterns.
    // If non-empty, only matching tools are exposed.
    // If empty (nil or []), all tools are candidates (subject to Deny).
    // Glob syntax: "*" matches any run of characters, "?" matches one.
    // +optional
    Allow []string `json:"allow,omitempty"`

    // Deny is a denylist of tool names and glob patterns.
    // Applied after Allow; Deny wins on conflict.
    // +optional
    Deny []string `json:"deny,omitempty"`
}

type ToolRouteStatus struct {
    // Url is the reachable URL assigned by the gateway implementation.
    // Agents consume this URL verbatim.
    // +optional
    Url string `json:"url,omitempty"`

    // Conditions: Accepted, Ready, ResolutionFailed (transient).
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

CEL validation on `spec.upstream` enforces exactly one of `toolServerRef` / `external` is set.

Example:

```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolRoute
metadata:
  name: github-readonly
  namespace: tools
spec:
  toolGatewayRef:
    name: tool-gateway
  upstream:
    toolServerRef:
      name: github-mcp
    # OR:
    # external:
    #   url: https://github-mcp.example.com/mcp
  toolFilter:
    allow: [search_issues, "get_*", "list_*"]
    deny:  ["*delete*", force_push]
```

### `AgentTool` -- breaking changes

`AgentTool` is reduced to three fields:

```go
type AgentTool struct {
    // Name is the unique identifier for this tool within the agent.
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`

    // ToolRouteRef references the ToolRoute that exposes the upstream MCP server.
    // This is the only way to attach a tool to an agent.
    // Namespace defaults to the Agent's namespace.
    // +kubebuilder:validation:Required
    ToolRouteRef corev1.ObjectReference `json:"toolRouteRef"`

    // PropagatedHeaders lists HTTP header names forwarded from incoming A2A
    // requests to the MCP tool server. Header names are case-insensitive.
    // +optional
    PropagatedHeaders []string `json:"propagatedHeaders,omitempty"`
}
```

**Removed:**

- `AgentTool.ToolServerRef` -- replaced by a ToolRoute that wraps the ToolServer.
- `AgentTool.Url` -- replaced by a ToolRoute with `upstream.external.url`.

### `ToolServer` -- breaking changes

**Removed:**

- `spec.toolGatewayRef` -- gateway attachment is now inferred from ToolRoutes that point at the ToolServer.
- `status.gatewayUrl` -- agents read their URL from `ToolRoute.status.url`.
- `status.toolGatewayRef` -- same reason.

**Kept:** all deployment fields (`image`, `transportType`, `port`, `env`, `replicas`, etc.) and `status.url` (the cluster-local Service URL, which the gateway implementation uses as the backend target).

**Default-gateway behavior removed.** The existing "ToolServer with no `toolGatewayRef` auto-attaches to a namespace's default ToolGateway" logic is deleted. A ToolServer not referenced by any ToolRoute simply isn't exposed anywhere. This is the correct model under the new design; migration notes call it out.

## Reconciliation Ownership

| Controller | Location | Watches | Writes |
|---|---|---|---|
| `AgentReconciler` | agent-runtime-operator | `Agent`, `ToolRoute` | Injects `AGENT_TOOLS` env on agent pod; `Agent.status` |
| `ToolRoute` reconciler | each tool-gateway implementation operator | `ToolRoute`, `ToolGateway`, `ToolServer` | Impl-specific gateway resources (routes/filters); `ToolRoute.status.url`, conditions |
| `ToolGateway` reconciler | each tool-gateway implementation operator | `ToolGateway` | Unchanged -- handles gateway lifecycle + guardrails |

`agent-runtime-operator` defines the `ToolRoute` CRD but does not reconcile it.

### Agent tool resolution

Replacing today's logic in `internal/controller/agent_tool.go`:

1. For each `tool.toolRouteRef`, fetch the `ToolRoute` (namespace defaulted to agent's).
2. Read `toolRoute.status.url`. If missing or the route is not `Ready`, record a per-tool resolution error.
3. Aggregate errors (same "failed to resolve N tool(s)" pattern as today) so all broken references surface at once.
4. Inject `{name, url, propagatedHeaders}` into the agent's `AGENT_TOOLS` env. The agent template sees the same payload shape it sees today; it doesn't know whether the URL terminates at a ToolRoute or a ToolServer.
5. Watch `ToolRoute` for status changes (direct replacement for `findAgentsReferencingToolServer`).

## Gateway Implementation Integration

Each tool-gateway implementation operator gains a `ToolRouteReconciler`. Pattern mirrors the existing `ToolGatewayReconciler`.

| Step | `tool-gateway-agentgateway` | `tool-gateway-envoy-ai-gateway` |
|---|---|---|
| Filter by class | Ignore ToolRoutes whose `toolGatewayRef` targets a gateway this controller doesn't own | same |
| Resolve upstream | `toolServerRef` -> cluster Service URL; `external.url` -> URL verbatim | same |
| Translate filter | agentgateway-native filter config | Envoy ext_proc filter config or `EnvoyExtensionPolicy` rules |
| Materialize route | `AgentgatewayBackend` / route config | `MCPRoute` + `HTTPRoute` |
| Status | Populate `ToolRoute.status.url` with the route's reachable URL | same |

**Breaking for `tool-gateway-agentgateway`.** The existing controller keys off `ToolServer.spec.toolGatewayRef`. With that field removed, it must be rewritten to watch `ToolRoute` instead. This is the main implementation cost.

**Filter translation is per-implementation.** The CRD stores `toolFilter` strings verbatim. No shared glob-matching helper in agent-runtime-operator.

## Status, Errors, Edge Cases

### ToolRoute conditions

| Condition | Meaning |
|---|---|
| `Accepted` | A gateway implementation recognized this ToolRoute's `toolGatewayRef` and took ownership. |
| `Ready` | Route is wired up, `status.url` is populated, traffic can flow. |
| `ResolutionFailed` | `toolServerRef` points to a non-existent or not-ready ToolServer. |

### Error scenarios

| Scenario | Surface | Behavior |
|---|---|---|
| ToolRoute's `toolGatewayRef` doesn't exist | `Accepted=False` | No implementation claims it; the route idles. |
| Agent references a missing ToolRoute | Agent's existing tool-resolution error path | "failed to resolve N tool(s)" |
| Agent references a ToolRoute whose `status.url` is empty | Same path | Transient; watch re-reconciles when route becomes Ready. |
| Upstream ToolServer deleted while routes point at it | `Ready=False`, `ResolutionFailed=True` | Agent re-reconciles via watch. |
| `allow` lists a name upstream doesn't expose | Silent (passive enforcement) | Agent gets "tool not available" at call time. Accepted trade-off for passive enforcement. |
| Invalid glob syntax | Implementation-specific; at worst, nothing matches | No security impact under allowlist semantics. |

### Deletion

`ToolRoute` has a finalizer owned by the gateway implementation operator, so gateway-side resources are removed before the route disappears from the API.

## Testing

Follows the three-tier pattern in `agent-runtime-operator/CLAUDE.md`.

**Unit tests** (co-located, standard Go testing):

- AgentTool resolution: `toolRouteRef` -> URL from `ToolRoute.status.url`; missing route; route not Ready.
- CEL validation on `spec.upstream` accepts exactly-one, rejects both/neither.
- Deepcopy regeneration check.

**Integration tests** (envtest):

- Agent controller rebuilds `AGENT_TOOLS` env when `ToolRoute.status.url` changes.
- Agent controller aggregates resolution errors across multiple tools.
- `ToolRoute` watch replaces today's `findAgentsReferencingToolServer` coverage.

**E2E tests** (Kind):

- Apply ToolServer + ToolRoute + Agent -> agent pod comes up with `AGENT_TOOLS` pointing at the route URL.
- External-upstream ToolRoute -> agent receives the external URL.

Filter-translation correctness and URL-routing correctness are tested in each gateway implementation's own repository.

## Migration

Breaking changes in `agent-runtime-operator/api/v1alpha1`:

| Change | Impact |
|---|---|
| `AgentTool.ToolServerRef` removed | Users must create a ToolRoute and switch to `toolRouteRef`. |
| `AgentTool.Url` removed | Users must create a ToolRoute with `upstream.external.url`. |
| `ToolServer.spec.toolGatewayRef` removed | Users must create a ToolRoute to expose a ToolServer; existing auto-attach to default ToolGateway no longer happens. |
| `ToolServer.status.gatewayUrl` / `status.toolGatewayRef` removed | Consumers that read these must switch to `ToolRoute.status.url`. |

No automated migration is provided. A manifest-conversion note belongs in release notes.

`tool-gateway-agentgateway` requires a coordinated release: new `ToolRouteReconciler` that watches `ToolRoute`, plus removal of the now-defunct `ToolServer.toolGatewayRef` handling.

## Follow-Up Work

In rough priority order:

1. **Global ceiling on `ToolServer`.** Add `ToolServer.spec.toolFilter` as a hard cap intersected with each route's filter. Needed once untrusted servers are a real concern.
2. **Consumer-identity enforcement.** Add `consumers: [{ serviceAccountRef | spiffeId }]` to ToolRoute; gateway verifies via TokenReview or mTLS. Closes the "any pod that knows the URL can call it" gap.
3. **Active upstream discovery.** ToolRoute reconciler calls `tools/list`, populates `status.effectiveTools`, surfaces warnings for filter names that don't exist upstream.
4. **Rich filter expressions.** Annotation-based (`readOnlyHint`), regex, CEL -- driven by real use cases.
5. **Per-route guardrails.** `ToolRoute.spec.guardrails` as an override/addition on top of `ToolGateway`-wide guardrails.
6. **Static / secret-injected headers** on `upstream.external` for machine-to-machine auth to external servers.
