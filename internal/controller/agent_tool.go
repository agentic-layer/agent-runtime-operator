/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	namespace := getNamespaceWithDefault(&tool.ToolRouteRef, parentNamespace)

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
