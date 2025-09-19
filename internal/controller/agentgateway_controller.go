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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/agentgateway"
	"github.com/agentic-layer/agent-runtime-operator/internal/constants"
)

// AgentGatewayReconciler reconciles an AgentGateway object
type AgentGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agentgateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agentgateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agentgateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AgentGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the AgentGateway instance
	var agentGateway runtimev1alpha1.AgentGateway
	if err := r.Get(ctx, req.NamespacedName, &agentGateway); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AgentGateway resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AgentGateway")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling AgentGateway", "name", agentGateway.Name, "namespace", agentGateway.Namespace)

	// Check if the provider has changed and delete owned resources if so
	currentProvider := agentGateway.Spec.Provider
	if currentProvider == "" {
		currentProvider = runtimev1alpha1.KrakenDProvider // Default provider
	}

	if agentGateway.Status.ObservedProvider != "" && agentGateway.Status.ObservedProvider != currentProvider {
		log.Info("Provider changed, deleting owned resources",
			"oldProvider", string(agentGateway.Status.ObservedProvider),
			"newProvider", string(currentProvider))

		if err := r.deleteOwnedResources(ctx, &agentGateway); err != nil {
			log.Error(err, "Failed to delete owned resources due to provider change")
			return ctrl.Result{}, err
		}
	}

	// Update the observed provider in status
	agentGateway.Status.ObservedProvider = currentProvider

	// Create provider instance
	provider, err := agentgateway.NewAgentGatewayProvider(currentProvider, r.Client, r.Scheme)
	if err != nil {
		log.Error(err, "Failed to create provider", "provider", currentProvider)
		return ctrl.Result{}, err
	}

	// Get all exposed agents
	exposedAgents, err := r.getExposedAgents(ctx)
	if err != nil {
		log.Error(err, "Failed to get exposed agents")
		return ctrl.Result{}, err
	}

	// Create all resources using the provider
	if err := provider.CreateAgentGatewayResources(ctx, &agentGateway, exposedAgents); err != nil {
		log.Error(err, "Failed to create agent gateway resources")
		return ctrl.Result{}, err
	}

	// Create Service (provider-independent)
	if err := r.ensureService(ctx, &agentGateway); err != nil {
		log.Error(err, "Failed to ensure Service for AgentGateway")
		return ctrl.Result{}, err
	}

	// Update the status to persist the observed provider
	if err := r.Status().Update(ctx, &agentGateway); err != nil {
		log.Error(err, "Failed to update AgentGateway status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deleteOwnedResources deletes all resources owned by the AgentGateway
func (r *AgentGatewayReconciler) deleteOwnedResources(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway) error {
	log := logf.FromContext(ctx)

	// Delete ConfigMaps
	var configMapList corev1.ConfigMapList
	if err := r.List(ctx, &configMapList, client.InNamespace(agentGateway.Namespace)); err != nil {
		return fmt.Errorf("failed to list ConfigMaps: %w", err)
	}
	for _, configMap := range configMapList.Items {
		for _, ownerRef := range configMap.OwnerReferences {
			if ownerRef.Kind == "AgentGateway" && ownerRef.Name == agentGateway.Name && ownerRef.UID == agentGateway.UID {
				log.Info("Deleting ConfigMap", "name", configMap.Name)
				if err := r.Delete(ctx, &configMap); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete ConfigMap %s: %w", configMap.Name, err)
				}
				break
			}
		}
	}

	// Delete Deployments
	var deploymentList appsv1.DeploymentList
	if err := r.List(ctx, &deploymentList, client.InNamespace(agentGateway.Namespace)); err != nil {
		return fmt.Errorf("failed to list Deployments: %w", err)
	}
	for _, deployment := range deploymentList.Items {
		for _, ownerRef := range deployment.OwnerReferences {
			if ownerRef.Kind == "AgentGateway" && ownerRef.Name == agentGateway.Name && ownerRef.UID == agentGateway.UID {
				log.Info("Deleting Deployment", "name", deployment.Name)
				if err := r.Delete(ctx, &deployment); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete Deployment %s: %w", deployment.Name, err)
				}
				break
			}
		}
	}

	// Delete Services
	var serviceList corev1.ServiceList
	if err := r.List(ctx, &serviceList, client.InNamespace(agentGateway.Namespace)); err != nil {
		return fmt.Errorf("failed to list Services: %w", err)
	}
	for _, service := range serviceList.Items {
		for _, ownerRef := range service.OwnerReferences {
			if ownerRef.Kind == "AgentGateway" && ownerRef.Name == agentGateway.Name && ownerRef.UID == agentGateway.UID {
				log.Info("Deleting Service", "name", service.Name)
				if err := r.Delete(ctx, &service); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete Service %s: %w", service.Name, err)
				}
				break
			}
		}
	}

	return nil
}

// getExposedAgents queries all Agent resources across all namespaces with Exposed: true
func (r *AgentGatewayReconciler) getExposedAgents(ctx context.Context) ([]*runtimev1alpha1.Agent, error) {
	var agentList runtimev1alpha1.AgentList
	if err := r.List(ctx, &agentList); err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	var exposedAgents []*runtimev1alpha1.Agent
	for i := range agentList.Items {
		agent := &agentList.Items[i]
		if agent.Spec.Exposed {
			exposedAgents = append(exposedAgents, agent)
		}
	}

	return exposedAgents, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.AgentGateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Named("agentgateway").
		Complete(r)
}

// ensureService creates or updates the AgentGateway service
func (r *AgentGatewayReconciler) ensureService(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway) error {
	log := logf.FromContext(ctx)

	serviceName := agentGateway.Name

	// First, generate the desired Service configuration
	desiredService, err := r.createServiceForAgentGateway(agentGateway, serviceName)
	if err != nil {
		return fmt.Errorf("failed to generate desired Service: %w", err)
	}

	// Try to get existing Service
	existingService := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agentGateway.Namespace}, existingService)

	if err != nil && errors.IsNotFound(err) {
		// Service doesn't exist, create it
		if err := r.Create(ctx, desiredService); err != nil {
			return fmt.Errorf("failed to create new Service %s: %w", serviceName, err)
		}
		log.Info("Successfully created Service", "name", serviceName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get Service: %w", err)
	}
	// Once created, the Service is immutable.

	return nil
}

// createServiceForAgentGateway creates a service for the AgentGateway
func (r *AgentGatewayReconciler) createServiceForAgentGateway(agentGateway *runtimev1alpha1.AgentGateway, serviceName string) (*corev1.Service, error) {
	labels := map[string]string{
		"app":      agentGateway.Name,
		"provider": string(agentGateway.Spec.Provider),
	}

	// Service selector should match deployment selector (stable labels only)
	selectorLabels := map[string]string{
		"app": agentGateway.Name,
	}

	// Create service port for the gateway (port 10000 named http)
	servicePort := corev1.ServicePort{
		Name:       "http",
		Port:       10000,
		TargetPort: intstr.FromInt32(constants.DefaultGatewayPort), // Target the agent gateway container port
		Protocol:   corev1.ProtocolTCP,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agentGateway.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    []corev1.ServicePort{servicePort},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	// Set AgentGateway as the owner of the Service
	if err := ctrl.SetControllerReference(agentGateway, service, r.Scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// serviceNeedsUpdate compares existing and desired Services to determine if an update is needed
func (r *AgentGatewayReconciler) serviceNeedsUpdate(existing, desired *corev1.Service) bool {
	// Compare labels
	if len(existing.Labels) != len(desired.Labels) {
		return true
	}

	for key, desiredValue := range desired.Labels {
		if existingValue, exists := existing.Labels[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	// Compare selector
	if len(existing.Spec.Selector) != len(desired.Spec.Selector) {
		return true
	}

	for key, desiredValue := range desired.Spec.Selector {
		if existingValue, exists := existing.Spec.Selector[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	// Compare ports
	if len(existing.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}

	// Create maps for easier comparison
	existingPortsMap := make(map[string]corev1.ServicePort)
	for _, port := range existing.Spec.Ports {
		existingPortsMap[port.Name] = port
	}

	for _, desiredPort := range desired.Spec.Ports {
		existingPort, exists := existingPortsMap[desiredPort.Name]
		if !exists {
			return true
		}

		// Compare key port fields
		if existingPort.Port != desiredPort.Port ||
			existingPort.TargetPort != desiredPort.TargetPort ||
			existingPort.Protocol != desiredPort.Protocol {
			return true
		}
	}

	// Compare service type
	if existing.Spec.Type != desired.Spec.Type {
		return true
	}

	return false
}
