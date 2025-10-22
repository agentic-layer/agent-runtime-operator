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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	aigatewayv1alpha1 "github.com/agentic-layer/ai-gateway-operator/api/v1alpha1"
)

// updateAgentStatusReady sets the agent status to Ready and updates the A2A URL and AiGatewayRef
func (r *AgentReconciler) updateAgentStatusReady(ctx context.Context, agent *runtimev1alpha1.Agent, aiGateway *aigatewayv1alpha1.AiGateway) error {
	// Compute the A2A URL if the agent has an A2A protocol
	agent.Status.Url = r.buildA2AAgentCardUrl(agent)

	// Set AiGatewayConnection if an AI Gateway is being used
	if aiGateway != nil {
		agent.Status.AiGatewayConnection = fmt.Sprintf("%s.%s", aiGateway.Name, aiGateway.Namespace)
	} else {
		agent.Status.AiGatewayConnection = "Not Connected"
	}

	// Set Ready condition to True
	meta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "Agent is ready",
		ObservedGeneration: agent.Generation,
	})

	if err := r.Status().Update(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	return nil
}

// updateAgentStatusNotReady sets the agent status to not Ready with a specific reason
func (r *AgentReconciler) updateAgentStatusNotReady(ctx context.Context, agent *runtimev1alpha1.Agent, reason, message string) error {
	// Clear the A2A URL since the agent is not ready
	agent.Status.Url = ""

	// Clear the AiGatewayConnection since the agent is not ready
	agent.Status.AiGatewayConnection = ""

	// Set Ready condition to False with the provided reason
	meta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: agent.Generation,
	})

	if err := r.Status().Update(ctx, agent); err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	return nil
}
