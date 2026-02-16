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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultToolGatewayNamespace = "tool-gateway"
)

// resolveToolGateway resolves the ToolGateway resource for a tool server and returns the ToolGateway object.
// The method follows this resolution strategy:
//  1. If toolServer.Spec.ToolGatewayRef is specified, looks up that specific ToolGateway resource
//  2. If toolServer.Spec.ToolGatewayRef is not specified, searches for any ToolGateway in the "tool-gateway" namespace
//  3. Returns nil (with no error) if no ToolGateway is found - this is a valid scenario
//
// Returns:
//   - ToolGateway object if a ToolGateway is found
//   - nil if no ToolGateway is found (not an error condition)
//   - error only for unexpected failures (e.g., API errors)
func (r *ToolServerReconciler) resolveToolGateway(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) (*runtimev1alpha1.ToolGateway, error) {
	// Case 1: Explicit ToolGatewayRef is specified
	if toolServer.Spec.ToolGatewayRef != nil {
		return r.resolveExplicitToolGateway(ctx, toolServer.Spec.ToolGatewayRef, toolServer.Namespace)
	}

	// Case 2: No explicit reference - search for default ToolGateway in tool-gateway namespace
	return r.resolveDefaultToolGateway(ctx)
}

// resolveExplicitToolGateway resolves a specific ToolGateway referenced by the tool server
func (r *ToolServerReconciler) resolveExplicitToolGateway(ctx context.Context, ref *corev1.ObjectReference, toolServerNamespace string) (*runtimev1alpha1.ToolGateway, error) {
	// Use namespace from ObjectReference, or default to tool server's namespace
	namespace := ref.Namespace
	if namespace == "" {
		namespace = toolServerNamespace
	}

	var toolGateway runtimev1alpha1.ToolGateway
	err := r.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}, &toolGateway)

	if err != nil {
		// If the ToolGateway CRD is not installed, return a clear error
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("ToolGateway CRD is not installed in the cluster")
		}
		return nil, fmt.Errorf("failed to resolve ToolGateway %s/%s: %w", namespace, ref.Name, err)
	}

	return &toolGateway, nil
}

// resolveDefaultToolGateway searches for any ToolGateway in the default tool-gateway namespace
func (r *ToolServerReconciler) resolveDefaultToolGateway(ctx context.Context) (*runtimev1alpha1.ToolGateway, error) {
	log := logf.FromContext(ctx)

	var toolGatewayList runtimev1alpha1.ToolGatewayList
	err := r.List(ctx, &toolGatewayList, client.InNamespace(defaultToolGatewayNamespace))
	if err != nil {
		// If the ToolGateway CRD is not installed, treat it as "no gateway found"
		if meta.IsNoMatchError(err) {
			log.Info("ToolGateway CRD is not installed, skipping default gateway resolution")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list ToolGateways in namespace %s: %w", defaultToolGatewayNamespace, err)
	}

	// No ToolGateway found - this is valid, return nil without error
	if len(toolGatewayList.Items) == 0 {
		return nil, nil
	}

	if len(toolGatewayList.Items) > 1 {
		log.Info("Multiple ToolGateways found, selecting first one",
			"selected", toolGatewayList.Items[0].Name,
			"count", len(toolGatewayList.Items))
	}

	// Return the first ToolGateway found
	toolGateway := toolGatewayList.Items[0]
	return &toolGateway, nil
}

// findToolServersReferencingToolGateway finds all tool servers that reference or would resolve to a given ToolGateway.
// This includes:
//  1. ToolServers with explicit ToolGatewayRef matching this gateway
//  2. ToolServers without explicit ToolGatewayRef that would resolve to this gateway as the default
func (r *ToolServerReconciler) findToolServersReferencingToolGateway(ctx context.Context, obj client.Object) []ctrl.Request {
	updatedGateway, ok := obj.(*runtimev1alpha1.ToolGateway)
	if !ok {
		return nil
	}

	log := logf.FromContext(ctx)

	// Find all tool servers in the cluster
	var toolServerList runtimev1alpha1.ToolServerList
	if err := r.List(ctx, &toolServerList); err != nil {
		log.Error(err, "Failed to list tool servers for ToolGateway watch")
		return nil
	}

	var requests []ctrl.Request

	// Check each tool server to see if it references this gateway
	for _, toolServer := range toolServerList.Items {
		shouldReconcile := false

		// Case 1: ToolServer has explicit ToolGatewayRef
		if toolServer.Spec.ToolGatewayRef != nil {
			// Resolve namespace (default to tool server's namespace if not specified)
			gatewayNamespace := toolServer.Spec.ToolGatewayRef.Namespace
			if gatewayNamespace == "" {
				gatewayNamespace = toolServer.Namespace
			}

			// Check if this tool server references the updated gateway
			if toolServer.Spec.ToolGatewayRef.Name == updatedGateway.Name && gatewayNamespace == updatedGateway.Namespace {
				shouldReconcile = true
				log.Info("Enqueuing tool server due to explicit ToolGatewayRef change",
					"toolServer", toolServer.Name,
					"namespace", toolServer.Namespace,
					"gateway", updatedGateway.Name,
					"gatewayNamespace", updatedGateway.Namespace)
			}
		} else {
			// Case 2: ToolServer has no explicit reference - would use default gateway resolution
			// Default resolution looks for any ToolGateway in the "tool-gateway" namespace
			if updatedGateway.Namespace == defaultToolGatewayNamespace {
				shouldReconcile = true
				log.Info("Enqueuing tool server due to default ToolGateway change",
					"toolServer", toolServer.Name,
					"namespace", toolServer.Namespace,
					"gateway", updatedGateway.Name,
					"gatewayNamespace", updatedGateway.Namespace)
			}
		}

		if shouldReconcile {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      toolServer.Name,
					Namespace: toolServer.Namespace,
				},
			})
		}
	}

	return requests
}
