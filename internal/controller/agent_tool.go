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
	corev1 "k8s.io/api/core/v1"
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

// resolveAllTools resolves every tool's URL via its upstream. Errors are aggregated so
// the caller sees all unresolved tools in a single message.
func (r *AgentReconciler) resolveAllTools(ctx context.Context, agent *runtimev1alpha1.Agent) (map[string]ResolvedTool, error) {
	resolved := make(map[string]ResolvedTool)
	var issues []string

	for _, tool := range agent.Spec.Tools {
		url, err := r.resolveToolUrl(ctx, tool, agent.Namespace)
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

// resolveToolUrl dispatches to the appropriate resolver based on which upstream field is set.
func (r *AgentReconciler) resolveToolUrl(ctx context.Context, tool runtimev1alpha1.AgentTool, parentNamespace string) (string, error) {
	switch {
	case tool.Upstream.ToolRouteRef != nil:
		return r.resolveToolRouteUrl(ctx, tool.Upstream.ToolRouteRef, parentNamespace)
	case tool.Upstream.ToolServerRef != nil:
		return r.resolveToolServerUrl(ctx, tool.Upstream.ToolServerRef, parentNamespace)
	case tool.Upstream.External != nil:
		url := strings.TrimSpace(tool.Upstream.External.Url)
		if url == "" {
			return "", fmt.Errorf("external upstream URL is empty")
		}
		return url, nil
	default:
		return "", fmt.Errorf("no upstream configured")
	}
}

// resolveToolRouteUrl fetches the ToolRoute and returns its status.url.
// Namespace defaults to parentNamespace if the ref doesn't set one.
func (r *AgentReconciler) resolveToolRouteUrl(ctx context.Context, ref *corev1.ObjectReference, parentNamespace string) (string, error) {
	namespace := getNamespaceWithDefault(ref, parentNamespace)
	var route runtimev1alpha1.ToolRoute
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &route); err != nil {
		return "", fmt.Errorf("failed to resolve ToolRoute %s/%s: %w", namespace, ref.Name, err)
	}
	if route.Status.Url == "" {
		return "", fmt.Errorf("ToolRoute %s/%s has no URL in its Status field", namespace, route.Name)
	}
	return route.Status.Url, nil
}

// resolveToolServerUrl fetches the ToolServer and returns its status.url.
// Namespace defaults to parentNamespace if the ref doesn't set one.
func (r *AgentReconciler) resolveToolServerUrl(ctx context.Context, ref *corev1.ObjectReference, parentNamespace string) (string, error) {
	namespace := getNamespaceWithDefault(ref, parentNamespace)
	var server runtimev1alpha1.ToolServer
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &server); err != nil {
		return "", fmt.Errorf("failed to resolve ToolServer %s/%s: %w", namespace, ref.Name, err)
	}
	if server.Status.Url == "" {
		return "", fmt.Errorf("ToolServer %s/%s has no URL in its Status field", namespace, server.Name)
	}
	return server.Status.Url, nil
}

// findAgentsReferencingToolRoute enqueues all Agents whose upstream.toolRouteRef matches the changed ToolRoute.
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
			if tool.Upstream.ToolRouteRef == nil || tool.Upstream.ToolRouteRef.Name != updated.Name {
				continue
			}
			ns := tool.Upstream.ToolRouteRef.Namespace
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

// findAgentsReferencingToolServer enqueues all Agents whose upstream.toolServerRef matches the changed ToolServer.
func (r *AgentReconciler) findAgentsReferencingToolServer(ctx context.Context, obj client.Object) []ctrl.Request {
	updated, ok := obj.(*runtimev1alpha1.ToolServer)
	if !ok {
		return nil
	}

	var agentList runtimev1alpha1.AgentList
	if err := r.List(ctx, &agentList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list agents for ToolServer watch")
		return nil
	}

	var requests []ctrl.Request
	for _, agent := range agentList.Items {
		for _, tool := range agent.Spec.Tools {
			if tool.Upstream.ToolServerRef == nil || tool.Upstream.ToolServerRef.Name != updated.Name {
				continue
			}
			ns := tool.Upstream.ToolServerRef.Namespace
			if ns == "" {
				ns = agent.Namespace
			}
			if ns != updated.Namespace {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace},
			})
			logf.FromContext(ctx).Info("Enqueuing agent due to ToolServer status change",
				"agent", agent.Name, "toolServer", updated.Name, "url", updated.Status.Url)
			break
		}
	}
	return requests
}
