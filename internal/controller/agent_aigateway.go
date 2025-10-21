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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	aigatewayv1alpha1 "github.com/agentic-layer/ai-gateway-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultAiGatewayNamespace = "ai-gateway"
)

// resolveAiGateway resolves the AiGateway resource for an agent and returns the AiGateway object.
// The method follows this resolution strategy:
//  1. If agent.Spec.AiGatewayRef is specified, looks up that specific AiGateway resource
//  2. If agent.Spec.AiGatewayRef is not specified, searches for any AiGateway in the "ai-gateway" namespace
//  3. Returns nil (with no error) if no AiGateway is found - this is a valid scenario
//
// Returns:
//   - AiGateway object if an AiGateway is found
//   - nil if no AiGateway is found (not an error condition)
//   - error only for unexpected failures (e.g., API errors)
func (r *AgentReconciler) resolveAiGateway(ctx context.Context, agent *runtimev1alpha1.Agent) (*aigatewayv1alpha1.AiGateway, error) {
	// Case 1: Explicit AiGatewayRef is specified
	if agent.Spec.AiGatewayRef != nil {
		return r.resolveExplicitAiGateway(ctx, agent.Spec.AiGatewayRef, agent.Namespace)
	}

	// Case 2: No explicit reference - search for default AiGateway in ai-gateway namespace
	return r.resolveDefaultAiGateway(ctx)
}

// resolveExplicitAiGateway resolves a specific AiGateway referenced by the agent
func (r *AgentReconciler) resolveExplicitAiGateway(ctx context.Context, ref *corev1.ObjectReference, agentNamespace string) (*aigatewayv1alpha1.AiGateway, error) {
	// Use namespace from ObjectReference, or default to agent's namespace
	namespace := ref.Namespace
	if namespace == "" {
		namespace = agentNamespace
	}

	var aiGateway aigatewayv1alpha1.AiGateway
	err := r.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}, &aiGateway)

	if err != nil {
		// If the AiGateway CRD is not installed, return a clear error
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("AiGateway CRD is not installed in the cluster")
		}
		return nil, fmt.Errorf("failed to resolve AiGateway %s/%s: %w", namespace, ref.Name, err)
	}

	return &aiGateway, nil
}

// resolveDefaultAiGateway searches for any AiGateway in the default ai-gateway namespace
func (r *AgentReconciler) resolveDefaultAiGateway(ctx context.Context) (*aigatewayv1alpha1.AiGateway, error) {
	log := logf.FromContext(ctx)

	var aiGatewayList aigatewayv1alpha1.AiGatewayList
	err := r.List(ctx, &aiGatewayList, client.InNamespace(defaultAiGatewayNamespace))
	if err != nil {
		// If the AiGateway CRD is not installed, treat it as "no gateway found"
		if meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list AiGateways in namespace %s: %w", defaultAiGatewayNamespace, err)
	}

	// No AiGateway found - this is valid, return nil without error
	if len(aiGatewayList.Items) == 0 {
		return nil, nil
	}

	if len(aiGatewayList.Items) > 1 {
		log.Info("Multiple AiGateways found, selecting first one",
			"selected", aiGatewayList.Items[0].Name,
			"count", len(aiGatewayList.Items))
	}

	// Return the first AiGateway found
	aiGateway := aiGatewayList.Items[0]
	return &aiGateway, nil
}

func (r *AgentReconciler) buildAiGatewayServiceUrl(aiGateway aigatewayv1alpha1.AiGateway) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", aiGateway.Name, aiGateway.Namespace, aiGateway.Spec.Port)
}
