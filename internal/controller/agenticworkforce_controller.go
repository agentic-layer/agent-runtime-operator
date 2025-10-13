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

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// AgenticWorkforceReconciler reconciles an AgenticWorkforce object
type AgenticWorkforceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agenticworkforces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agenticworkforces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agenticworkforces/finalizers,verbs=update
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *AgenticWorkforceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the AgenticWorkforce instance
	var workforce runtimev1alpha1.AgenticWorkforce
	if err := r.Get(ctx, req.NamespacedName, &workforce); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AgenticWorkforce resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AgenticWorkforce")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling AgenticWorkforce", "name", workforce.Name, "namespace", workforce.Namespace)

	// Validate entry point agents
	allAgentsReady, missingAgents := r.validateEntryPointAgents(ctx, &workforce)

	// Collect transitive agents and tools
	transitiveAgents, transitiveTools := r.collectTransitiveAgentsAndTools(ctx, &workforce)

	// Update status condition based on validation
	if allAgentsReady {
		meta.SetStatusCondition(&workforce.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: workforce.Generation,
			Reason:             "AllAgentsReady",
			Message:            "All entry point agents are available",
		})
	} else {
		meta.SetStatusCondition(&workforce.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: workforce.Generation,
			Reason:             "AgentsMissing",
			Message:            fmt.Sprintf("Missing agents: %v", missingAgents),
		})
	}

	// Update transitive agents and tools
	workforce.Status.TransitiveAgents = transitiveAgents
	workforce.Status.TransitiveTools = transitiveTools

	if err := r.Status().Update(ctx, &workforce); err != nil {
		log.Error(err, "Failed to update AgenticWorkforce status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled AgenticWorkforce",
		"transitiveAgents", len(transitiveAgents),
		"transitiveTools", len(transitiveTools))

	return ctrl.Result{}, nil
}

// validateEntryPointAgents checks if all referenced entry point agents exist
func (r *AgenticWorkforceReconciler) validateEntryPointAgents(ctx context.Context, workforce *runtimev1alpha1.AgenticWorkforce) (bool, []string) {
	log := logf.FromContext(ctx)
	allReady := true
	var missingAgents []string

	for _, agentRef := range workforce.Spec.EntryPointAgents {
		// Skip nil references
		if agentRef == nil {
			log.Info("Skipping nil agent reference")
			continue
		}

		namespace := agentRef.Namespace
		if namespace == "" {
			namespace = workforce.Namespace
		}

		var agent runtimev1alpha1.Agent
		err := r.Get(ctx, types.NamespacedName{
			Name:      agentRef.Name,
			Namespace: namespace,
		}, &agent)

		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Entry point agent not found", "agent", agentRef.Name, "namespace", namespace)
				allReady = false
				missingAgents = append(missingAgents, fmt.Sprintf("%s/%s", namespace, agentRef.Name))
			} else {
				log.Error(err, "Failed to get entry point agent", "agent", agentRef.Name, "namespace", namespace)
				allReady = false
				missingAgents = append(missingAgents, fmt.Sprintf("%s/%s", namespace, agentRef.Name))
			}
		}
	}

	return allReady, missingAgents
}

// collectTransitiveAgentsAndTools recursively collects all agents and tools from entry-point agents
func (r *AgenticWorkforceReconciler) collectTransitiveAgentsAndTools(ctx context.Context, workforce *runtimev1alpha1.AgenticWorkforce) ([]runtimev1alpha1.TransitiveAgent, []string) {
	log := logf.FromContext(ctx)

	visitedAgents := make(map[string]runtimev1alpha1.TransitiveAgent)
	allTools := make(map[string]bool)

	// Process each entry-point agent
	for _, agentRef := range workforce.Spec.EntryPointAgents {
		// Skip nil references
		if agentRef == nil {
			log.Info("Skipping nil agent reference during traversal")
			continue
		}

		namespace := agentRef.Namespace
		if namespace == "" {
			namespace = workforce.Namespace
		}

		agentKey := fmt.Sprintf("%s/%s", namespace, agentRef.Name)
		if err := r.traverseAgent(ctx, namespace, agentRef.Name, visitedAgents, allTools); err != nil {
			log.Error(err, "Failed to traverse agent", "agent", agentKey)
			// Continue with other agents even if one fails
		}
	}

	// Convert map to slice for consistent output
	agents := make([]runtimev1alpha1.TransitiveAgent, 0, len(visitedAgents))
	for _, agent := range visitedAgents {
		agents = append(agents, agent)
	}

	tools := make([]string, 0, len(allTools))
	for tool := range allTools {
		tools = append(tools, tool)
	}

	return agents, tools
}

// traverseAgent recursively traverses an agent and its sub-agents, collecting all agents and tools
func (r *AgenticWorkforceReconciler) traverseAgent(ctx context.Context, namespace, name string, visitedAgents map[string]runtimev1alpha1.TransitiveAgent, allTools map[string]bool) error {
	log := logf.FromContext(ctx)

	agentKey := fmt.Sprintf("%s/%s", namespace, name)

	// Avoid infinite loops by checking if we've already visited this agent
	if _, exists := visitedAgents[agentKey]; exists {
		return nil
	}

	// Mark this agent as visited
	visitedAgents[agentKey] = runtimev1alpha1.TransitiveAgent{
		Name:      name,
		Namespace: namespace,
	}

	// Fetch the agent
	var agent runtimev1alpha1.Agent
	err := r.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &agent)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Agent not found during traversal", "agent", agentKey)
			return nil // Don't fail the whole operation
		}
		return fmt.Errorf("failed to get agent %s: %w", agentKey, err)
	}

	// Collect tools from this agent
	for _, tool := range agent.Spec.Tools {
		allTools[tool.Url] = true
	}

	// Recursively traverse sub-agents
	for _, subAgent := range agent.Spec.SubAgents {
		if subAgent.AgentRef != nil {
			// Cluster agent reference
			subNamespace := subAgent.AgentRef.Namespace
			if subNamespace == "" {
				subNamespace = agent.Namespace // Use parent agent's namespace as default
			}
			subName := subAgent.AgentRef.Name

			if err := r.traverseAgent(ctx, subNamespace, subName, visitedAgents, allTools); err != nil {
				log.Error(err, "Failed to traverse sub-agent", "subAgent", fmt.Sprintf("%s/%s", subNamespace, subName))
				// Continue with other sub-agents
			}
		} else if subAgent.Url != "" {
			// Remote agent - record as remote agent with URL
			remoteAgentKey := subAgent.Url
			visitedAgents[remoteAgentKey] = runtimev1alpha1.TransitiveAgent{
				Name: subAgent.Name,
				Url:  subAgent.Url,
			}
		}
	}

	return nil
}

// findWorkforcesReferencingAgent finds all AgenticWorkforce resources that reference a given agent
func (r *AgenticWorkforceReconciler) findWorkforcesReferencingAgent(ctx context.Context, obj client.Object) []ctrl.Request {
	agent, ok := obj.(*runtimev1alpha1.Agent)
	if !ok {
		return nil
	}

	// Find all workforces
	var workforceList runtimev1alpha1.AgenticWorkforceList
	if err := r.List(ctx, &workforceList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list workforces for agent watch")
		return nil
	}

	var requests []ctrl.Request
	agentKey := fmt.Sprintf("%s/%s", agent.Namespace, agent.Name)

	// Check each workforce to see if it references this agent
	for _, workforce := range workforceList.Items {
		// Check if this agent is directly referenced as an entry point
		for _, entryRef := range workforce.Spec.EntryPointAgents {
			// Skip nil references
			if entryRef == nil {
				continue
			}

			entryNamespace := entryRef.Namespace
			if entryNamespace == "" {
				entryNamespace = workforce.Namespace
			}
			entryKey := fmt.Sprintf("%s/%s", entryNamespace, entryRef.Name)

			if entryKey == agentKey {
				// This workforce directly references the agent
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      workforce.Name,
						Namespace: workforce.Namespace,
					},
				})
				logf.FromContext(ctx).Info("Enqueuing workforce due to agent change",
					"workforce", workforce.Name,
					"agent", agentKey)
				break
			}
		}

		// Also check transitive references (if the agent appears in status.transitiveAgents)
		for _, transitiveAgent := range workforce.Status.TransitiveAgents {
			// Match cluster agents by name and namespace
			if transitiveAgent.Name == agent.Name && transitiveAgent.Namespace == agent.Namespace {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      workforce.Name,
						Namespace: workforce.Namespace,
					},
				})
				logf.FromContext(ctx).Info("Enqueuing workforce due to transitive agent change",
					"workforce", workforce.Name,
					"agent", agentKey)
				break
			}
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgenticWorkforceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.AgenticWorkforce{}).
		Watches(
			&runtimev1alpha1.Agent{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkforcesReferencingAgent),
		).
		Named("agenticworkforce").
		Complete(r)
}
