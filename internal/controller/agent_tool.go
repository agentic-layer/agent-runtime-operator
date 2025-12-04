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

// resolveAllTools resolves all tool ToolServer URLs and returns a map of name to URL.
// Collects all resolution errors and returns them together, allowing the caller to see
// all missing tools at once rather than failing on the first error.
func (r *AgentReconciler) resolveAllTools(ctx context.Context, agent *runtimev1alpha1.Agent) (map[string]string, error) {
	resolved := make(map[string]string)
	var issues []string

	for _, tool := range agent.Spec.Tools {
		url, err := r.resolveToolServerUrl(ctx, tool, agent.Namespace)
		if err != nil {
			issues = append(issues, fmt.Sprintf("tool %q: %v", tool.Name, err))
		} else {
			resolved[tool.Name] = url
		}
	}

	if len(issues) > 0 {
		return resolved, fmt.Errorf("failed to resolve %d tool(s): %s", len(issues), strings.Join(issues, "; "))
	}

	return resolved, nil
}

// resolveToolServerUrl resolves a Tool configuration to its actual URL.
// For remote tools (with URL): returns the URL directly
// For cluster tools (with toolServerRef): looks up the ToolServer resource and uses its status.url
// If namespace is not specified in toolServerRef, defaults to the parent agent's namespace
func (r *AgentReconciler) resolveToolServerUrl(ctx context.Context, tool runtimev1alpha1.AgentTool, parentNamespace string) (string, error) {
	// If URL is provided, this is a remote tool - use URL directly
	if tool.Url != "" {
		return tool.Url, nil
	}

	// This is a cluster ToolServer reference - resolve by looking up the resource
	if tool.ToolServerRef == nil {
		return "", fmt.Errorf("tool has neither url nor toolServerRef specified")
	}

	// Use namespace from ObjectReference, or default to parent namespace
	namespace := GetNamespaceWithDefault(tool.ToolServerRef, parentNamespace)

	var referencedToolServer runtimev1alpha1.ToolServer
	err := r.Get(ctx, types.NamespacedName{
		Name:      tool.ToolServerRef.Name,
		Namespace: namespace,
	}, &referencedToolServer)

	if err != nil {
		return "", fmt.Errorf("failed to resolve ToolServer %s/%s: %w", namespace, tool.ToolServerRef.Name, err)
	}

	// Use the URL from the toolServer's status (populated by the controller)
	if referencedToolServer.Status.Url == "" {
		return "", fmt.Errorf("ToolServer %s/%s has no URL in its Status field (may not be ready or transport is stdio)", namespace, tool.ToolServerRef.Name)
	}

	return referencedToolServer.Status.Url, nil
}

// findAgentsReferencingToolServer finds all agents that reference a given ToolServer
func (r *AgentReconciler) findAgentsReferencingToolServer(ctx context.Context, obj client.Object) []ctrl.Request {
	updatedToolServer, ok := obj.(*runtimev1alpha1.ToolServer)
	if !ok {
		return nil
	}

	// Find all agents that reference this ToolServer
	var agentList runtimev1alpha1.AgentList
	if err := r.List(ctx, &agentList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list agents for ToolServer watch")
		return nil
	}

	var requests []ctrl.Request
	// Enqueue agents that reference this ToolServer
	for _, agent := range agentList.Items {
		for _, tool := range agent.Spec.Tools {
			// Skip tools with direct URLs (no ToolServerRef)
			if tool.ToolServerRef == nil {
				continue
			}

			// Check if this agent references the updated ToolServer
			if tool.ToolServerRef.Name == updatedToolServer.Name {
				// Use helper to get namespace
				toolServerNamespace := GetNamespaceWithDefault(tool.ToolServerRef, agent.Namespace)

				if toolServerNamespace == updatedToolServer.Namespace {
					// This agent references the updated ToolServer - enqueue it
					requests = append(requests, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      agent.Name,
							Namespace: agent.Namespace,
						},
					})
					logf.FromContext(ctx).Info("Enqueuing agent due to ToolServer status change",
						"agent", agent.Name,
						"toolServer", updatedToolServer.Name,
						"updatedToolServerURL", updatedToolServer.Status.Url)
					break // Found a match, no need to check other tools
				}
			}
		}
	}

	return requests
}
