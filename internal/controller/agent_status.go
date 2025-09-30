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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// resolveSubAgents checks if all subAgents can be resolved and returns a list of errors
func (r *AgentReconciler) resolveSubAgents(ctx context.Context, agent *runtimev1alpha1.Agent) []string {
	var errorMessages []string

	for _, subAgent := range agent.Spec.SubAgents {
		_, err := r.resolveSubAgentUrl(ctx, subAgent, agent.Namespace)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", subAgent.Name, err))
		}
	}

	return errorMessages
}

// updateCondition updates or adds a condition to the agent's status
func (r *AgentReconciler) updateCondition(agent *runtimev1alpha1.Agent, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: agent.Generation,
	}

	// Find existing condition
	for i, existingCondition := range agent.Status.Conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition
			agent.Status.Conditions[i] = condition
			agent.Status.Conditions[i].LastTransitionTime = metav1.Now()
			return
		}
	}

	// Add new condition
	condition.LastTransitionTime = metav1.Now()
	agent.Status.Conditions = append(agent.Status.Conditions, condition)
}

// updateAgentStatus updates the agent's status with computed fields like the A2A URL and subAgent resolution status
func (r *AgentReconciler) updateAgentStatus(ctx context.Context, agent *runtimev1alpha1.Agent, subAgentErrors []string) error {
	// Compute the A2A URL if the agent has an A2A protocol
	newUrl := r.buildA2AAgentCardUrl(agent)
	agent.Status.Url = newUrl

	// Set SubAgentsResolved condition
	if len(subAgentErrors) == 0 {
		r.updateCondition(agent, "SubAgentsResolved", metav1.ConditionTrue,
			"AllResolved", "All subAgents resolved successfully")
	} else {
		r.updateCondition(agent, "SubAgentsResolved", metav1.ConditionFalse,
			"ResolutionFailed", fmt.Sprintf("Failed to resolve subAgents: %s", strings.Join(subAgentErrors, "; ")))
	}

	if err := r.Status().Update(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	return nil
}
