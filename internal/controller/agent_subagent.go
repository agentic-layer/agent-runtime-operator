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

// ResolvedSubAgent contains the resolved information for a sub-agent
type ResolvedSubAgent struct {
	// Url is the resolved agent card URL
	Url string
	// InteractionType specifies how to interact with this sub-agent ("transfer" or "tool_call")
	InteractionType string
}

// resolveAllSubAgents resolves all subAgent URLs and returns a map of name to ResolvedSubAgent.
// Collects all resolution errors and returns them together, allowing the caller to see
// all missing subAgents at once rather than failing on the first error.
func (r *AgentReconciler) resolveAllSubAgents(ctx context.Context, agent *runtimev1alpha1.Agent) (map[string]ResolvedSubAgent, error) {
	resolved := make(map[string]ResolvedSubAgent)
	var issues []string

	for _, subAgent := range agent.Spec.SubAgents {
		url, err := r.resolveSubAgentUrl(ctx, subAgent, agent.Namespace)
		if err != nil {
			issues = append(issues, fmt.Sprintf("subAgent %q: %v", subAgent.Name, err))
		} else {
			resolved[subAgent.Name] = ResolvedSubAgent{
				Url:             url,
				InteractionType: subAgent.InteractionType,
			}
		}
	}

	if len(issues) > 0 {
		return resolved, fmt.Errorf("failed to resolve %d subAgent(s): %s", len(issues), strings.Join(issues, "; "))
	}

	return resolved, nil
}

// resolveSubAgentUrl resolves a SubAgent configuration to its actual URL.
// For remote agents (with URL): returns the URL directly
// For cluster agents (with agentRef): looks up the Agent resource and uses its status.url
// If namespace is not specified in agentRef, defaults to the parent agent's namespace
func (r *AgentReconciler) resolveSubAgentUrl(ctx context.Context, subAgent runtimev1alpha1.SubAgent, parentNamespace string) (string, error) {
	// If URL is provided, this is a remote agent - use URL directly
	if subAgent.Url != "" {
		return subAgent.Url, nil
	}

	// This is a cluster agent reference - resolve by looking up the Agent resource
	if subAgent.AgentRef == nil {
		return "", fmt.Errorf("subAgent has neither url nor agentRef specified")
	}

	// Use namespace from ObjectReference, or default to parent namespace
	namespace := GetNamespaceWithDefault(subAgent.AgentRef, parentNamespace)

	var referencedAgent runtimev1alpha1.Agent
	err := r.Get(ctx, types.NamespacedName{
		Name:      subAgent.AgentRef.Name,
		Namespace: namespace,
	}, &referencedAgent)

	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster agent %s/%s: %w", namespace, subAgent.AgentRef.Name, err)
	}

	// Use the URL from the agent's status (populated by the controller)
	if referencedAgent.Status.Url == "" {
		return "", fmt.Errorf("cluster Agent %s/%s has no URL in its Status field (may not be ready or have A2A protocol)", namespace, subAgent.AgentRef.Name)
	}

	return referencedAgent.Status.Url, nil
}

// findAgentsReferencingSubAgent finds all agents that reference a given agent as a subAgent
func (r *AgentReconciler) findAgentsReferencingSubAgent(ctx context.Context, obj client.Object) []ctrl.Request {
	updatedAgent, ok := obj.(*runtimev1alpha1.Agent)
	if !ok {
		return nil
	}

	// Find all agents that reference this agent as a subAgent
	var agentList runtimev1alpha1.AgentList
	if err := r.List(ctx, &agentList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list agents for subAgent watch")
		return nil
	}

	var requests []ctrl.Request
	// Enqueue agents that reference this agent
	for _, agent := range agentList.Items {
		for _, subAgent := range agent.Spec.SubAgents {
			// Check if this is a cluster agent reference (using agentRef)
			if subAgent.AgentRef != nil && subAgent.AgentRef.Name == updatedAgent.Name {
				// Check namespace match (use namespace from ObjectReference)
				subAgentNamespace := GetNamespaceWithDefault(subAgent.AgentRef, agent.Namespace)

				if subAgentNamespace == updatedAgent.Namespace {
					// This agent references the updated agent - enqueue it
					requests = append(requests, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Name:      agent.Name,
							Namespace: agent.Namespace,
						},
					})
					logf.FromContext(ctx).Info("Enqueuing parent agent due to subAgent status change",
						"parent", agent.Name,
						"subAgent", updatedAgent.Name,
						"updatedAgentURL", updatedAgent.Status.Url)
					break // Found a match, no need to check other subAgents
				}
			}
		}
	}

	return requests
}
