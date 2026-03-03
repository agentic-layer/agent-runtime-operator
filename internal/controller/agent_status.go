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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// updateAgentStatusReady sets the agent status to Ready and updates the A2A URL and AiGatewayRef
func (r *AgentReconciler) updateAgentStatusReady(ctx context.Context, agent *runtimev1alpha1.Agent, aiGateway *runtimev1alpha1.AiGateway) error {
	log := logf.FromContext(ctx)
	log.V(1).Info("Updating agent status to Ready")
	// Compute the A2A URL if the agent has an A2A protocol
	agent.Status.Url = buildA2AAgentCardUrl(agent)

	// Set AiGatewayRef if an AI Gateway is being used
	if aiGateway != nil {
		agent.Status.AiGatewayRef = &corev1.ObjectReference{
			APIVersion: aiGateway.APIVersion,
			Kind:       aiGateway.Kind,
			Name:       aiGateway.Name,
			Namespace:  aiGateway.Namespace,
		}
	} else {
		agent.Status.AiGatewayRef = nil
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

// getDeploymentReadiness inspects the deployment status conditions and returns an aggregated
// readiness state. Returns (true, "", "") when the deployment is available, or (false, reason,
// message) with a user-friendly, aggregated message when it is not.
func getDeploymentReadiness(deployment *appsv1.Deployment) (bool, string, string) {
	// ReplicaFailure indicates pod creation failures (e.g. missing referenced Secrets/ConfigMaps)
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
			return false, "DeploymentNotReady", "Deployment not ready: Referenced resources missing"
		}
	}

	// ProgressDeadlineExceeded indicates the deployment could not complete within the deadline
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing &&
			condition.Status == corev1.ConditionFalse &&
			condition.Reason == "ProgressDeadlineExceeded" {
			return false, "DeploymentNotReady", "Deployment not ready: Deployment failed to progress"
		}
	}

	// Available condition reports whether minimum replicas are up
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable {
			if condition.Status == corev1.ConditionTrue {
				return true, "", ""
			}
			return false, "DeploymentNotReady", "Deployment not ready: Insufficient replicas available"
		}
	}

	// No conditions yet – deployment was just created and has not been evaluated
	return false, "DeploymentNotReady", "Deployment not ready: Waiting for deployment to become available"
}

// updateAgentStatusDeploymentNotReady sets the Ready condition to False due to a deployment-level
// issue while still populating the URL and AiGatewayRef so that the agent remains discoverable.
func (r *AgentReconciler) updateAgentStatusDeploymentNotReady(ctx context.Context, agent *runtimev1alpha1.Agent, aiGateway *runtimev1alpha1.AiGateway, reason, message string) error {
	log := logf.FromContext(ctx)
	log.V(1).Info("Updating agent status: deployment not ready", "reason", reason)

	// Keep the A2A URL so the agent remains discoverable (spec is valid, pods just aren't ready)
	agent.Status.Url = buildA2AAgentCardUrl(agent)

	// Keep AiGatewayRef – the gateway association is still valid
	if aiGateway != nil {
		agent.Status.AiGatewayRef = &corev1.ObjectReference{
			APIVersion: aiGateway.APIVersion,
			Kind:       aiGateway.Kind,
			Name:       aiGateway.Name,
			Namespace:  aiGateway.Namespace,
		}
	} else {
		agent.Status.AiGatewayRef = nil
	}

	// Set Ready condition to False with the deployment-specific reason
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

// updateAgentStatusNotReady sets the agent status to not Ready with a specific reason
func (r *AgentReconciler) updateAgentStatusNotReady(ctx context.Context, agent *runtimev1alpha1.Agent, reason, message string) error {
	log := logf.FromContext(ctx)
	log.V(1).Info("Updating agent status to Not Ready")

	// Clear the A2A URL since the agent is not ready
	agent.Status.Url = ""

	// Clear the AiGatewayRef since the agent is not ready
	agent.Status.AiGatewayRef = nil

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
