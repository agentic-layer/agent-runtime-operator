# Tool Filtering via ToolRoute — `agent-runtime-operator` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `ToolRoute` CRD and adapt `Agent` + `ToolServer` in `agent-runtime-operator` to the new model, where `ToolRoute` is the single attachment path for tools.

**Architecture:** Introduce a new namespace-scoped CRD `ToolRoute` that owns routing + filter spec. Remove `ToolServer.spec.toolGatewayRef` (and related status fields). Reduce `AgentTool` to `{name, toolRouteRef, propagatedHeaders}`. Agent tool resolution now reads `ToolRoute.status.url`. `agent-runtime-operator` does not reconcile `ToolRoute`; that work lives in each tool-gateway implementation operator (separate plan).

**Tech Stack:** Go 1.24, controller-runtime, kubebuilder, envtest, Ginkgo/Gomega. Repo: ``. Group/version: `runtime.agentic-layer.ai/v1alpha1`.

**Related spec:** `docs/superpowers/specs/2026-04-22-tool-filtering-toolroute-design.md`

**Scope note:** This plan covers `agent-runtime-operator` only. The `tool-gateway-agentgateway` rewrite (new `ToolRouteReconciler`, removal of old `ToolServer.toolGatewayRef` handling) is a separate plan. Until both ship together, end-to-end tool traffic breaks — coordinate releases.

---

## File Structure

**Created:**
- `api/v1alpha1/toolroute_types.go` — CRD types
- `config/samples/runtime_v1alpha1_toolroute.yaml` — sample
- `docs/modules/tool-routes/partials/how-to-guide.adoc` — user guide
- `internal/controller/toolroute_validation_test.go` — envtest for CRD CEL validation
- `test/e2e/toolroute_e2e_test.go` — e2e test

**Modified:**
- `api/v1alpha1/agent_types.go` — `AgentTool` struct reduced
- `api/v1alpha1/toolserver_types.go` — drop gateway fields
- `api/v1alpha1/zz_generated.deepcopy.go` — regenerated
- `config/crd/bases/*.yaml` — regenerated
- `internal/controller/agent_tool.go` — resolution via ToolRoute
- `internal/controller/agent_tool_test.go` — updated tests
- `internal/controller/agent_reconciler.go` — watch ToolRoute, not ToolServer
- `internal/controller/toolserver_reconciler.go` — drop gateway-URL logic
- `internal/controller/toolserver_reconciler_test.go` — drop tests for removed behavior
- `internal/webhook/v1alpha1/agent_webhook.go` — simplified `validateTool`
- `internal/webhook/v1alpha1/agent_webhook_test.go` — updated cases
- `config/samples/kustomization.yaml` — register new sample
- `docs/modules/agent-runtime/partials/reference.adoc` — add ToolRoute entry
- `docs/modules/tool-servers/partials/how-to-guide.adoc` — update to use ToolRoute
- `CLAUDE.md` — document the new CRD

**Deleted:**
- `internal/controller/toolserver_toolgateway.go`
- `internal/controller/toolserver_toolgateway_test.go`

---

## Task 1: Scaffold `ToolRoute` CRD type

**Files:**
- Create: `api/v1alpha1/toolroute_types.go`
- Modify (regenerated): `api/v1alpha1/zz_generated.deepcopy.go`
- Modify (regenerated): `config/crd/bases/runtime.agentic-layer.ai_toolroutes.yaml` (new file)

- [ ] **Step 1: Create the types file**

Create `api/v1alpha1/toolroute_types.go`:

```go
/*
Copyright 2025 Agentic Layer.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToolRouteSpec defines the desired state of ToolRoute.
type ToolRouteSpec struct {
	// ToolGatewayRef identifies the ToolGateway hosting this route.
	// Namespace defaults to the ToolRoute's namespace if not specified.
	// +kubebuilder:validation:Required
	ToolGatewayRef corev1.ObjectReference `json:"toolGatewayRef"`

	// Upstream specifies the MCP server this route proxies.
	// Exactly one of ToolServerRef or External must be set.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="(has(self.toolServerRef) ? 1 : 0) + (has(self.external) ? 1 : 0) == 1",message="exactly one of toolServerRef or external must be set"
	Upstream ToolRouteUpstream `json:"upstream"`

	// ToolFilter restricts which tools are exposed through this route.
	// If nil, all tools pass through unfiltered.
	// +optional
	ToolFilter *ToolFilter `json:"toolFilter,omitempty"`
}

// ToolRouteUpstream describes the upstream MCP server for a route.
// Exactly one of ToolServerRef or External must be set.
type ToolRouteUpstream struct {
	// ToolServerRef references a cluster-local ToolServer.
	// Namespace defaults to the ToolRoute's namespace if not specified.
	// Mutually exclusive with External.
	// +optional
	ToolServerRef *corev1.ObjectReference `json:"toolServerRef,omitempty"`

	// External describes a remote MCP server reachable at an HTTP URL.
	// Mutually exclusive with ToolServerRef.
	// +optional
	External *ExternalUpstream `json:"external,omitempty"`
}

// ExternalUpstream describes a remote MCP server reachable at an HTTP URL.
type ExternalUpstream struct {
	// Url is the HTTP/HTTPS endpoint of the external MCP server.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	Url string `json:"url"`
}

// ToolFilter restricts which tools are exposed through a route.
// Matching uses glob syntax: "*" matches any run of characters, "?" matches one.
// Deny is applied after Allow and wins on conflict.
type ToolFilter struct {
	// Allow is an allowlist of tool names and glob patterns.
	// If non-empty, only matching tools are exposed.
	// If empty or nil, all tools are candidates (subject to Deny).
	// +optional
	Allow []string `json:"allow,omitempty"`

	// Deny is a denylist of tool names and glob patterns.
	// Applied after Allow; Deny wins on conflict.
	// +optional
	Deny []string `json:"deny,omitempty"`
}

// ToolRouteStatus defines the observed state of ToolRoute.
type ToolRouteStatus struct {
	// Url is the reachable URL assigned by the gateway implementation.
	// Agents consume this URL verbatim.
	// +optional
	Url string `json:"url,omitempty"`

	// Conditions represent the latest available observations of the route's state.
	// Known condition types: Accepted, Ready, ResolutionFailed.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Gateway",type="string",JSONPath=".spec.toolGatewayRef.name"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ToolRoute is the Schema for the toolroutes API.
type ToolRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolRouteSpec   `json:"spec,omitempty"`
	Status ToolRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolRouteList contains a list of ToolRoute.
type ToolRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolRoute{}, &ToolRouteList{})
}
```

- [ ] **Step 2: Regenerate manifests and deepcopy**

Run:
```bash
make manifests generate
```

Expected: `zz_generated.deepcopy.go` gains `ToolRoute*` methods; `config/crd/bases/runtime.agentic-layer.ai_toolroutes.yaml` is created.

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Verify CRD manifest contains the CEL rule**

Run:
```bash
grep -A 2 "x-kubernetes-validations" config/crd/bases/runtime.agentic-layer.ai_toolroutes.yaml
```

Expected: shows the `has(self.toolServerRef) != has(self.external)` rule (or its `?:` equivalent as emitted by controller-gen).

- [ ] **Step 5: Commit**

```bash
  git add api/v1alpha1/toolroute_types.go \
          api/v1alpha1/zz_generated.deepcopy.go \
          config/crd/bases/runtime.agentic-layer.ai_toolroutes.yaml && \
  git commit -m "feat: add ToolRoute CRD type"
```

---

## Task 2: CEL validation envtest for `ToolRoute`

**Files:**
- Create: `internal/controller/toolroute_validation_test.go`

- [ ] **Step 1: Write the test**

Create `internal/controller/toolroute_validation_test.go`:

```go
package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolRoute CEL validation", func() {
	ctx := context.Background()
	ns := "default"

	newRoute := func(name string, u runtimev1alpha1.ToolRouteUpstream) *runtimev1alpha1.ToolRoute {
		return &runtimev1alpha1.ToolRoute{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: runtimev1alpha1.ToolRouteSpec{
				ToolGatewayRef: corev1.ObjectReference{Name: "tg"},
				Upstream:       u,
			},
		}
	}

	It("accepts upstream with only toolServerRef", func() {
		r := newRoute("valid-tsref", runtimev1alpha1.ToolRouteUpstream{
			ToolServerRef: &corev1.ObjectReference{Name: "ts"},
		})
		Expect(k8sClient.Create(ctx, r)).To(Succeed())
	})

	It("accepts upstream with only external.url", func() {
		r := newRoute("valid-ext", runtimev1alpha1.ToolRouteUpstream{
			External: &runtimev1alpha1.ExternalUpstream{Url: "https://example.com/mcp"},
		})
		Expect(k8sClient.Create(ctx, r)).To(Succeed())
	})

	It("rejects upstream with both toolServerRef and external", func() {
		r := newRoute("invalid-both", runtimev1alpha1.ToolRouteUpstream{
			ToolServerRef: &corev1.ObjectReference{Name: "ts"},
			External:      &runtimev1alpha1.ExternalUpstream{Url: "https://example.com/mcp"},
		})
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one of toolServerRef or external"))
	})

	It("rejects upstream with neither toolServerRef nor external", func() {
		r := newRoute("invalid-empty", runtimev1alpha1.ToolRouteUpstream{})
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one of toolServerRef or external"))
	})
})
```

- [ ] **Step 2: Run the suite, expect 4 specs pass**

Run:
```bash
go test ./internal/controller/ -v -ginkgo.focus="ToolRoute CEL validation"
```

Expected: 4 specs pass. (If they fail with "unknown type", verify the CRD is registered in `suite_test.go`; it should already be since `init()` registers in step 1.)

- [ ] **Step 3: Commit**

```bash
  git add internal/controller/toolroute_validation_test.go && \
  git commit -m "test: CEL validation for ToolRoute upstream one-of"
```

---

## Task 3: Replace `AgentTool.{ToolServerRef, Url}` with `ToolRouteRef`

**Files:**
- Modify: `api/v1alpha1/agent_types.go:77-102` (AgentTool struct)
- Modify (regenerated): `api/v1alpha1/zz_generated.deepcopy.go`
- Modify (regenerated): `config/crd/bases/runtime.agentic-layer.ai_agents.yaml`

- [ ] **Step 1: Replace the `AgentTool` struct**

In `api/v1alpha1/agent_types.go`, replace the entire `AgentTool` struct definition with:

```go
// AgentTool defines configuration for attaching a tool to an agent via a ToolRoute.
type AgentTool struct {
	// Name is the unique identifier for this tool within the agent.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ToolRouteRef references the ToolRoute that exposes the upstream MCP server.
	// This is the only way to attach a tool to an agent.
	// If Namespace is not specified, defaults to the same namespace as the current Agent.
	// +kubebuilder:validation:Required
	ToolRouteRef corev1.ObjectReference `json:"toolRouteRef"`

	// PropagatedHeaders is a list of HTTP header names that should be propagated from incoming
	// A2A requests to the MCP tool server. Header names are case-insensitive.
	// If not specified or empty, no headers are propagated.
	// +optional
	PropagatedHeaders []string `json:"propagatedHeaders,omitempty"`
}
```

Note: we use `corev1.ObjectReference` (non-pointer) with `Required`, plus `MinLength=1` on the `Name` field is enforced via the referenced object's own schema (controller-gen flatens `ObjectReference`).

- [ ] **Step 2: Regenerate**

Run:
```bash
make manifests generate
```

Expected: `agents.yaml` CRD no longer has `toolServerRef` / `url` fields under `spec.tools[]`; has `toolRouteRef` required.

- [ ] **Step 3: Attempt to build — expect failures in callers**

Run:
```bash
go build ./... 2>&1 | head -40
```

Expected: compilation errors in `internal/controller/agent_tool.go` (references `tool.Url`, `tool.ToolServerRef`) and in `internal/webhook/v1alpha1/agent_webhook.go` (`validateTool`). Fix these in Tasks 4 and 5 — we'll commit at the end of Task 5 once the build is green.

Do NOT commit yet.

---

## Task 4: Rewrite agent tool resolution to use `ToolRoute`

**Files:**
- Modify: `internal/controller/agent_tool.go` (full rewrite)
- Modify: `internal/controller/agent_tool_test.go`

- [ ] **Step 1: Write the new unit tests first**

Replace the helper `createAgentWithToolServerRef` and related test cases in `internal/controller/agent_tool_test.go` with `ToolRoute`-based equivalents. Add these specs (keep the existing outer `Describe` block; replace the inner test bodies that referenced `ToolServerRef` / `Url` on `AgentTool`):

```go
// helper
func createToolRoute(ctx context.Context, k8sClient client.Client, name, namespace, url string) *runtimev1alpha1.ToolRoute {
	tr := &runtimev1alpha1.ToolRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: runtimev1alpha1.ToolRouteSpec{
			ToolGatewayRef: corev1.ObjectReference{Name: "tg"},
			Upstream: runtimev1alpha1.ToolRouteUpstream{
				External: &runtimev1alpha1.ExternalUpstream{Url: "https://upstream.example.com/mcp"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, tr)).To(Succeed())
	tr.Status.Url = url
	Expect(k8sClient.Status().Update(ctx, tr)).To(Succeed())
	return tr
}

It("resolves a tool via ToolRoute.status.url", func() {
	createToolRoute(ctx, k8sClient, "tr-1", "default", "https://gw.local/r/default/tr-1/mcp")
	agent := createAgentWithTools(ctx, k8sClient, "a1", []runtimev1alpha1.AgentTool{{
		Name:         "t1",
		ToolRouteRef: corev1.ObjectReference{Name: "tr-1"},
	}})
	resolved, err := reconciler.resolveAllTools(ctx, agent)
	Expect(err).NotTo(HaveOccurred())
	Expect(resolved).To(HaveKey("t1"))
	Expect(resolved["t1"].Url).To(Equal("https://gw.local/r/default/tr-1/mcp"))
})

It("returns aggregated error when a tool's ToolRoute is missing", func() {
	agent := createAgentWithTools(ctx, k8sClient, "a2", []runtimev1alpha1.AgentTool{{
		Name:         "t1",
		ToolRouteRef: corev1.ObjectReference{Name: "does-not-exist"},
	}})
	_, err := reconciler.resolveAllTools(ctx, agent)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(`tool "t1"`))
	Expect(err.Error()).To(ContainSubstring("failed to resolve"))
})

It("returns error when referenced ToolRoute has empty status.url", func() {
	tr := &runtimev1alpha1.ToolRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tr-noready", Namespace: "default"},
		Spec: runtimev1alpha1.ToolRouteSpec{
			ToolGatewayRef: corev1.ObjectReference{Name: "tg"},
			Upstream: runtimev1alpha1.ToolRouteUpstream{
				External: &runtimev1alpha1.ExternalUpstream{Url: "https://upstream.example.com/mcp"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, tr)).To(Succeed())
	agent := createAgentWithTools(ctx, k8sClient, "a3", []runtimev1alpha1.AgentTool{{
		Name:         "t1",
		ToolRouteRef: corev1.ObjectReference{Name: "tr-noready"},
	}})
	_, err := reconciler.resolveAllTools(ctx, agent)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("has no URL"))
})

It("defaults ToolRoute namespace to the Agent's namespace", func() {
	createToolRoute(ctx, k8sClient, "tr-default-ns", "default", "https://gw/default/tr-default-ns")
	agent := createAgentWithTools(ctx, k8sClient, "a4", []runtimev1alpha1.AgentTool{{
		Name:         "t1",
		ToolRouteRef: corev1.ObjectReference{Name: "tr-default-ns"}, // no Namespace
	}})
	resolved, err := reconciler.resolveAllTools(ctx, agent)
	Expect(err).NotTo(HaveOccurred())
	Expect(resolved["t1"].Url).To(Equal("https://gw/default/tr-default-ns"))
})
```

Delete the tests that covered `AgentTool.Url` and `AgentTool.ToolServerRef` paths — they no longer apply.

- [ ] **Step 2: Replace `agent_tool.go` with ToolRoute-based resolution**

Replace the whole file content with:

```go
/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ResolvedTool represents a fully resolved tool with its URL and propagated headers.
type ResolvedTool struct {
	Url               string
	PropagatedHeaders []string
}

// resolveAllTools resolves every tool's URL via its ToolRoute. Errors are aggregated so
// the caller sees all unresolved tools in a single message.
func (r *AgentReconciler) resolveAllTools(ctx context.Context, agent *runtimev1alpha1.Agent) (map[string]ResolvedTool, error) {
	resolved := make(map[string]ResolvedTool)
	var issues []string

	for _, tool := range agent.Spec.Tools {
		url, err := r.resolveToolRouteUrl(ctx, tool, agent.Namespace)
		if err != nil {
			issues = append(issues, fmt.Sprintf("tool %q: %v", tool.Name, err))
			continue
		}
		resolved[tool.Name] = ResolvedTool{
			Url:               url,
			PropagatedHeaders: tool.PropagatedHeaders,
		}
	}

	if len(issues) > 0 {
		return resolved, fmt.Errorf("failed to resolve %d tool(s): %s", len(issues), strings.Join(issues, "; "))
	}
	return resolved, nil
}

// resolveToolRouteUrl fetches the ToolRoute referenced by the tool and returns status.url.
// Namespace defaults to the parent Agent's namespace if the ref doesn't set one.
func (r *AgentReconciler) resolveToolRouteUrl(ctx context.Context, tool runtimev1alpha1.AgentTool, parentNamespace string) (string, error) {
	if tool.ToolRouteRef.Name == "" {
		return "", fmt.Errorf("toolRouteRef.name is empty")
	}

	namespace := tool.ToolRouteRef.Namespace
	if namespace == "" {
		namespace = parentNamespace
	}

	var route runtimev1alpha1.ToolRoute
	if err := r.Get(ctx, types.NamespacedName{Name: tool.ToolRouteRef.Name, Namespace: namespace}, &route); err != nil {
		return "", fmt.Errorf("failed to resolve ToolRoute %s/%s: %w", namespace, tool.ToolRouteRef.Name, err)
	}
	if route.Status.Url == "" {
		return "", fmt.Errorf("ToolRoute %s/%s has no URL in its Status field", namespace, route.Name)
	}
	return route.Status.Url, nil
}

// findAgentsReferencingToolRoute enqueues all Agents that reference the changed ToolRoute.
// Replaces findAgentsReferencingToolServer. Triggered by watches on ToolRoute status changes.
func (r *AgentReconciler) findAgentsReferencingToolRoute(ctx context.Context, obj client.Object) []ctrl.Request {
	updated, ok := obj.(*runtimev1alpha1.ToolRoute)
	if !ok {
		return nil
	}

	var agentList runtimev1alpha1.AgentList
	if err := r.List(ctx, &agentList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list agents for ToolRoute watch")
		return nil
	}

	var requests []ctrl.Request
	for _, agent := range agentList.Items {
		for _, tool := range agent.Spec.Tools {
			if tool.ToolRouteRef.Name != updated.Name {
				continue
			}
			ns := tool.ToolRouteRef.Namespace
			if ns == "" {
				ns = agent.Namespace
			}
			if ns != updated.Namespace {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace},
			})
			logf.FromContext(ctx).Info("Enqueuing agent due to ToolRoute status change",
				"agent", agent.Name, "toolRoute", updated.Name, "url", updated.Status.Url)
			break
		}
	}
	return requests
}
```

- [ ] **Step 3: Run the updated tests — expect pass**

Run:
```bash
go test ./internal/controller/ -v -ginkgo.focus="resolveAllTools|ToolRoute"
```

Expected: all new specs pass.

Do NOT commit yet — webhooks and controller watches still need to be fixed in Tasks 5-6 before `go build ./...` goes green.

---

## Task 5: Simplify Agent admission webhook

**Files:**
- Modify: `internal/webhook/v1alpha1/agent_webhook.go:231-260` (`validateTool`)
- Modify: `internal/webhook/v1alpha1/agent_webhook_test.go`

- [ ] **Step 1: Replace `validateTool`**

Replace the `validateTool` function body in `agent_webhook.go` with:

```go
// validateTool validates a single AgentTool entry.
// Stateless validation only; cross-object references (e.g., existence of ToolRoute) are enforced at reconcile time.
func (v *AgentCustomValidator) validateTool(tool runtimev1alpha1.AgentTool, index int) []*field.Error {
	var errs []*field.Error

	if tool.ToolRouteRef.Name == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "tools").Index(index).Child("toolRouteRef", "name"),
			"toolRouteRef.name must be set",
		))
	}

	return errs
}
```

- [ ] **Step 2: Update webhook tests**

In `agent_webhook_test.go`, remove test cases covering `ToolServerRef`/`Url` one-of validation on `AgentTool`. Replace with:

```go
It("rejects a tool with empty toolRouteRef.name", func() {
	agent := &runtimev1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "default"},
		Spec: runtimev1alpha1.AgentSpec{
			Tools: []runtimev1alpha1.AgentTool{{Name: "t1"}}, // ToolRouteRef.Name empty
		},
	}
	_, err := validator.ValidateCreate(ctx, agent)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("toolRouteRef.name must be set"))
})

It("accepts a tool with a valid toolRouteRef", func() {
	agent := &runtimev1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "good", Namespace: "default"},
		Spec: runtimev1alpha1.AgentSpec{
			Tools: []runtimev1alpha1.AgentTool{{
				Name:         "t1",
				ToolRouteRef: corev1.ObjectReference{Name: "some-route"},
			}},
		},
	}
	_, err := validator.ValidateCreate(ctx, agent)
	Expect(err).NotTo(HaveOccurred())
})
```

- [ ] **Step 3: Run webhook tests**

Run:
```bash
go test ./internal/webhook/v1alpha1/ -v
```

Expected: all tests pass.

---

## Task 6: Rewire Agent controller watch to `ToolRoute`

**Files:**
- Modify: `internal/controller/agent_reconciler.go:365-395` (`SetupWithManager`)

- [ ] **Step 1: Replace the ToolServer watch with a ToolRoute watch**

In `agent_reconciler.go`'s `SetupWithManager`, replace the `Watches(&runtimev1alpha1.ToolServer{}, ...)` block with:

```go
		Watches(
			&runtimev1alpha1.ToolRoute{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsReferencingToolRoute),
		).
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run full unit/integration suite**

Run:
```bash
make test
```

Expected: all tests pass (both new ToolRoute specs and pre-existing suites unrelated to ToolServer-gateway-URL logic).

- [ ] **Step 4: Commit Tasks 3-6 together**

These tasks couldn't commit in isolation because they break compilation. Commit the combined change:

```bash
  git add api/v1alpha1/agent_types.go \
          api/v1alpha1/zz_generated.deepcopy.go \
          config/crd/bases/runtime.agentic-layer.ai_agents.yaml \
          internal/controller/agent_tool.go \
          internal/controller/agent_tool_test.go \
          internal/controller/agent_reconciler.go \
          internal/webhook/v1alpha1/agent_webhook.go \
          internal/webhook/v1alpha1/agent_webhook_test.go && \
  git commit -m "feat!: attach agent tools exclusively via ToolRouteRef

BREAKING: AgentTool.ToolServerRef and AgentTool.Url are removed.
Agents must reference a ToolRoute via AgentTool.ToolRouteRef."
```

---

## Task 7: Strip `ToolServer` of gateway concerns

**Files:**
- Modify: `api/v1alpha1/toolserver_types.go` (remove `ToolGatewayRef` from spec; `GatewayUrl`, `ToolGatewayRef` from status)
- Modify: `api/v1alpha1/zz_generated.deepcopy.go` (regenerated)
- Modify: `config/crd/bases/runtime.agentic-layer.ai_toolservers.yaml` (regenerated)
- Modify: `internal/controller/toolserver_reconciler.go` (remove gateway-URL status population)
- Modify: `internal/controller/toolserver_reconciler_test.go` (drop tests for removed behavior)
- Delete: `internal/controller/toolserver_toolgateway.go`
- Delete: `internal/controller/toolserver_toolgateway_test.go`

- [ ] **Step 1: Remove the fields from the ToolServer types**

In `toolserver_types.go`:

- Delete the `ToolGatewayRef *corev1.ObjectReference` field from `ToolServerSpec` (lines 90-95 of the current file).
- Delete the `GatewayUrl string` and `ToolGatewayRef *corev1.ObjectReference` fields from `ToolServerStatus` (lines 118-129).
- Delete the `"Tool Gateway"` printcolumn marker above the `ToolServer` type.

Remove any now-unused imports this exposes (if `corev1` is still used by `ImagePullSecrets`, keep it).

- [ ] **Step 2: Delete the toolserver_toolgateway helper files**

```bash
rm internal/controller/toolserver_toolgateway.go \
   internal/controller/toolserver_toolgateway_test.go
```

- [ ] **Step 3: Remove gateway-URL logic from `toolserver_reconciler.go`**

In `toolserver_reconciler.go`, find the status-update function `updateToolServerStatusReady` (around line 331) and delete the blocks that populate `Status.ToolGatewayRef` and `Status.GatewayUrl` (including the corresponding `nil`-out blocks on failure). Any calls that resolved a ToolGateway for the server should be deleted entirely.

Also remove `Watches(&runtimev1alpha1.ToolGateway{}, ...)` entries from the ToolServer reconciler's `SetupWithManager` if present, along with any `findToolServersReferencingToolGateway` helper.

- [ ] **Step 4: Remove the corresponding test cases**

In `toolserver_reconciler_test.go`, delete:
- Any `It("should resolve explicit ToolGatewayRef …")` specs.
- Any assertions on `updatedToolServer.Status.ToolGatewayRef` or `Status.GatewayUrl`.
- Any test helper that injects a `ToolGatewayRef` into `ToolServerSpec`.

- [ ] **Step 5: Regenerate**

Run:
```bash
make manifests generate
```

- [ ] **Step 6: Build and test**

Run:
```bash
go build ./... && make test
```

Expected: no errors, all remaining tests pass.

- [ ] **Step 7: Commit**

```bash
  git add api/v1alpha1/toolserver_types.go \
          api/v1alpha1/zz_generated.deepcopy.go \
          config/crd/bases/runtime.agentic-layer.ai_toolservers.yaml \
          internal/controller/toolserver_reconciler.go \
          internal/controller/toolserver_reconciler_test.go && \
  git rm internal/controller/toolserver_toolgateway.go \
         internal/controller/toolserver_toolgateway_test.go && \
  git commit -m "feat!: ToolServer no longer manages gateway attachment

BREAKING: ToolServer.spec.toolGatewayRef, status.gatewayUrl, and
status.toolGatewayRef are removed. Gateway attachment is now expressed
via ToolRoute.spec.upstream.toolServerRef."
```

---

## Task 8: Sample manifest and docs

**Files:**
- Create: `config/samples/runtime_v1alpha1_toolroute.yaml`
- Modify: `config/samples/kustomization.yaml`
- Modify: `docs/modules/agent-runtime/partials/reference.adoc`
- Modify: `docs/modules/tool-servers/partials/how-to-guide.adoc`
- Create: `docs/modules/tool-routes/partials/how-to-guide.adoc`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Create the sample**

Create `config/samples/runtime_v1alpha1_toolroute.yaml`:

```yaml
---
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolRoute
metadata:
  labels:
    app.kubernetes.io/name: agent-runtime-operator
    app.kubernetes.io/managed-by: kustomize
  name: example-toolroute
  namespace: test-tool-servers
spec:
  toolGatewayRef:
    name: example-toolgateway
  upstream:
    toolServerRef:
      name: example-http-toolserver
  toolFilter:
    allow: ["get_*", "list_*"]
    deny: ["*delete*"]
```

- [ ] **Step 2: Register the sample in kustomization.yaml**

Open `config/samples/kustomization.yaml` and add `- runtime_v1alpha1_toolroute.yaml` to the `resources:` list, alphabetically adjacent to `runtime_v1alpha1_toolserver.yaml`.

- [ ] **Step 3: Add ToolRoute to reference.adoc**

In `docs/modules/agent-runtime/partials/reference.adoc`, add within the CRD list:

```adoc
* *ToolRoute*: https://github.com/agentic-layer/agent-runtime-operator/blob/main/config/crd/bases/runtime.agentic-layer.ai_toolroutes.yaml
```

Place it alphabetically next to `ToolServer`.

- [ ] **Step 4: Rewrite the tool-servers how-to to use ToolRoute**

Edit `docs/modules/tool-servers/partials/how-to-guide.adoc`. In the "Using Tool Servers with Agents" section, replace the example that uses `toolServerRef` on the agent with one that uses `toolRouteRef`, and add a minimal companion `ToolRoute`. Keep the rest of the quick-start sections intact.

Updated snippet:

```asciidoc
== Using Tool Servers with Agents

Tool servers are exposed to agents through a ToolRoute. Define a ToolRoute to expose a
ToolServer via a ToolGateway, then reference the route from the agent:

[source,yaml]
----
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolRoute
metadata:
  name: context7-route
spec:
  toolGatewayRef:
    name: tool-gateway
  upstream:
    toolServerRef:
      name: context7-toolserver
---
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: coding-assistant
spec:
  framework: google-adk
  tools:
    - name: context7_tools
      toolRouteRef:
        name: context7-route
  protocols:
    - type: A2A
  replicas: 1
  env:
    - name: GEMINI_API_KEY
      value: "your-gemini-api-key-here"
----
```

- [ ] **Step 5: Create a ToolRoute how-to guide**

Create `docs/modules/tool-routes/partials/how-to-guide.adoc`:

```asciidoc
= Create a Tool Route

A ToolRoute exposes a tool server (cluster-local or external) through a ToolGateway, with
optional per-route filtering on which tools are visible.

IMPORTANT: Before following this guide, install the Agent Runtime Operator and have a
ToolGateway available. See the Agent Runtime Operator guide for installation.

== Expose a cluster-local ToolServer

[source,yaml]
----
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
  toolFilter:
    allow: [search_issues, "get_*", "list_*"]
    deny:  ["*delete*", force_push]
----

== Expose an external MCP server

[source,yaml]
----
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolRoute
metadata:
  name: github-readonly
  namespace: tools
spec:
  toolGatewayRef:
    name: tool-gateway
  upstream:
    external:
      url: https://github-mcp.example.com/mcp
  toolFilter:
    allow: [search_issues, "get_*", "list_*"]
----

== Filter syntax

Patterns match tool names using glob syntax:

* `*` matches any run of characters
* `?` matches exactly one character

`allow` defines the candidate set (if nil or empty, every tool is a candidate).
`deny` is applied after `allow` and wins on conflict.

== Reference a ToolRoute from an Agent

[source,yaml]
----
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: summarizer-agent
spec:
  tools:
    - name: github
      toolRouteRef:
        name: github-readonly
        namespace: tools
----
```

- [ ] **Step 6: Update CLAUDE.md**

In `CLAUDE.md`, add a `ToolRoute CRD` bullet to the **Core Components** section, adjacent to the `ToolServer CRD` entry:

```markdown
- **ToolRoute CRD** (`api/v1alpha1/toolroute_types.go`): Defines the ToolRoute custom resource for per-consumer exposure of tool servers through a ToolGateway:
  - Upstream reference (cluster ToolServer or external URL)
  - Tool filter (name + glob allow/deny)
  - Status URL populated by the gateway implementation operator
  - Not reconciled by agent-runtime-operator — each tool-gateway implementation owns its reconciliation
```

Remove or update any mention of `toolGatewayRef` on ToolServer and `gatewayUrl` status. In the "Agent Controller" section, update the tool-resolution description to reference `ToolRoute.status.url` rather than `ToolServer.status.gatewayUrl`.

- [ ] **Step 7: Commit**

```bash
  git add config/samples/runtime_v1alpha1_toolroute.yaml \
          config/samples/kustomization.yaml \
          docs/modules/agent-runtime/partials/reference.adoc \
          docs/modules/tool-servers/partials/how-to-guide.adoc \
          docs/modules/tool-routes/partials/how-to-guide.adoc \
          CLAUDE.md && \
  git commit -m "docs: sample manifest and user guides for ToolRoute"
```

---

## Task 9: E2E test for Agent + ToolRoute + ToolServer

**Files:**
- Create: `test/e2e/toolroute_e2e_test.go`

Pattern reference: existing `test/e2e/toolserver_e2e_test.go` and `test/e2e/agent_e2e_test.go`.

- [ ] **Step 1: Write the e2e test**

Create `test/e2e/toolroute_e2e_test.go`. Use the same bootstrap utilities as `toolserver_e2e_test.go`. The test should:

1. Create a namespace `e2e-toolroute`.
2. Apply a ToolServer (http transport, simple placeholder image is fine — e2e doesn't exercise the MCP protocol here).
3. Apply a ToolRoute referencing that ToolServer; set `ToolGatewayRef` to a pre-existing or placeholder ToolGateway.
4. Simulate the gateway impl by patching `ToolRoute.status.url = "http://fake-gw/e2e/mcp"` and marking the `Ready` condition true (this test does not require a gateway controller; it asserts the agent-runtime-operator agent-side logic consumes the status correctly).
5. Apply an Agent with `tools[0].toolRouteRef` pointing at the ToolRoute.
6. Wait for the Agent Deployment to exist and its pod spec's `AGENT_TOOLS` env to contain `http://fake-gw/e2e/mcp`.
7. Clean up.

Skeleton (fill in using helpers from `e2e_suite_test.go`):

```go
package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolRoute E2E", Ordered, func() {
	const ns = "e2e-toolroute"

	BeforeAll(func() {
		ensureNamespace(ns) // existing helper in e2e_suite_test.go
	})

	AfterAll(func() {
		deleteNamespace(ns)
	})

	It("wires AGENT_TOOLS to ToolRoute.status.url", func() {
		// 1. Apply ToolServer
		ts := &runtimev1alpha1.ToolServer{
			ObjectMeta: metav1.ObjectMeta{Name: "ts-1", Namespace: ns},
			Spec: runtimev1alpha1.ToolServerSpec{
				Protocol:      "mcp",
				TransportType: "http",
				Image:         "mcp/context7:latest",
				Port:          8080,
			},
		}
		Expect(k8sClient.Create(ctx, ts)).To(Succeed())

		// 2. Apply ToolRoute
		tr := &runtimev1alpha1.ToolRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "tr-1", Namespace: ns},
			Spec: runtimev1alpha1.ToolRouteSpec{
				ToolGatewayRef: corev1.ObjectReference{Name: "tg-fake"},
				Upstream: runtimev1alpha1.ToolRouteUpstream{
					ToolServerRef: &corev1.ObjectReference{Name: "ts-1"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, tr)).To(Succeed())

		// 3. Simulate gateway impl populating status
		tr.Status.Url = "http://fake-gw/e2e/mcp"
		Expect(k8sClient.Status().Update(ctx, tr)).To(Succeed())

		// 4. Apply Agent
		agent := &runtimev1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{Name: "a-1", Namespace: ns},
			Spec: runtimev1alpha1.AgentSpec{
				Framework: "google-adk",
				Replicas:  ptrInt32(1),
				Tools: []runtimev1alpha1.AgentTool{{
					Name:         "gh",
					ToolRouteRef: corev1.ObjectReference{Name: "tr-1"},
				}},
				Protocols: []runtimev1alpha1.AgentProtocol{{Type: "A2A"}},
			},
		}
		Expect(k8sClient.Create(ctx, agent)).To(Succeed())

		// 5. Assert AGENT_TOOLS contains the URL
		Eventually(func(g Gomega) {
			depl := getAgentDeployment(ns, "a-1") // existing helper
			env := envVars(depl.Spec.Template.Spec.Containers[0])
			g.Expect(env).To(HaveKeyWithValue("AGENT_TOOLS", ContainSubstring("http://fake-gw/e2e/mcp")))
		}, 60*time.Second, 2*time.Second).Should(Succeed())
	})
})

func ptrInt32(i int32) *int32 { return &i }
```

Where `ensureNamespace`, `deleteNamespace`, `getAgentDeployment`, `envVars`, `k8sClient`, `ctx` already exist in `e2e_suite_test.go` — use those directly; if a helper is missing, add a small one beside existing ones rather than duplicating bootstrap.

- [ ] **Step 2: Run the e2e suite**

Run:
```bash
make test-e2e
```

Expected: all e2e specs pass, including the new one. The existing `toolserver_e2e_test.go` may need minor updates if it asserted on the removed `Status.GatewayUrl` or `Status.ToolGatewayRef` — fix those by deleting the stale assertions.

- [ ] **Step 3: Commit**

```bash
  git add test/e2e/toolroute_e2e_test.go test/e2e/toolserver_e2e_test.go && \
  git commit -m "test(e2e): verify Agent consumes ToolRoute.status.url"
```

---

## Final verification

- [ ] **Step 1: Run the full test matrix**

```bash
make lint && make test && make test-e2e
```

Expected: green across lint, unit/integration, and e2e.

- [ ] **Step 2: Verify CRD list**

```bash
ls config/crd/bases/ | grep toolroutes
```

Expected: `runtime.agentic-layer.ai_toolroutes.yaml` present.

- [ ] **Step 3: Verify removed files are gone**

```bash
ls internal/controller/ | grep toolserver_toolgateway || echo "gone (good)"
```

Expected: `gone (good)`.
