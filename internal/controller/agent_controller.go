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
	"github.com/agentic-layer/agent-runtime-operator/internal/equality"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	agentContainerName = "agent"
	agentCardEndpoint  = "/.well-known/agent-card.json"
	a2AProtocol        = "A2A"
)

// AgentReconciler reconciles a Agent object
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Agent instance
	var agent runtimev1alpha1.Agent
	if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Agent resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Agent")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Agent")

	// Check if deployment already exists
	deployment := &appsv1.Deployment{}
	deploymentName := agent.Name
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agent.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		// Deployment does not exist, create it
		log.Info("Creating new Deployment")

		deployment, err = r.createDeploymentForAgent(ctx, &agent, deploymentName)
		if err != nil {
			log.Error(err, "Failed to create deployment for Agent")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}

		log.Info("Successfully created Deployment")
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	} else {
		// Deployment exists, check if it needs to be updated
		desiredDeployment, err := r.createDeploymentForAgent(ctx, &agent, deploymentName)
		if err != nil {
			log.Error(err, "Failed to create desired deployment spec for Agent")
			return ctrl.Result{}, err
		}

		if r.needsDeploymentUpdate(deployment, desiredDeployment) {
			log.Info("Updating existing Deployment")

			// Update deployment spec - only fields we manage
			deployment.Spec.Replicas = desiredDeployment.Spec.Replicas
			deployment.Labels = desiredDeployment.Labels
			deployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
			// Note: Selector is immutable in Kubernetes, so we don't update it

			// Update agent container by finding it by name (addressing PR feedback)
			if err := r.updateAgentContainer(deployment, desiredDeployment); err != nil {
				log.Error(err, "Failed to update agent container")
				return ctrl.Result{}, err
			}

			if err := r.Update(ctx, deployment); err != nil {
				log.Error(err, "Failed to update Deployment")
				return ctrl.Result{}, err
			}

			log.Info("Successfully updated Deployment")
		}
	}

	// Create service if protocols are defined
	if len(agent.Spec.Protocols) > 0 {
		service := &corev1.Service{}
		serviceName := agent.Name
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agent.Namespace}, service)

		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating new Service")

			service, err = r.createServiceForAgent(&agent, serviceName)
			if err != nil {
				log.Error(err, "Failed to create service for Agent")
				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create new Service")
				return ctrl.Result{}, err
			}

			log.Info("Successfully created Service")
		} else if err != nil {
			log.Error(err, "Failed to get Service")
			return ctrl.Result{}, err
		} else {
			// Service exists, check if it needs to be updated
			desiredService, err := r.createServiceForAgent(&agent, serviceName)
			if err != nil {
				log.Error(err, "Failed to create desired service spec for Agent")
				return ctrl.Result{}, err
			}

			if r.needsServiceUpdate(service, desiredService) {
				log.Info("Updating existing Service")

				// Update service spec
				// Note: Selector should remain stable, only update ports and labels
				service.Spec.Ports = desiredService.Spec.Ports
				service.Labels = desiredService.Labels

				if err := r.Update(ctx, service); err != nil {
					log.Error(err, "Failed to update Service")
					return ctrl.Result{}, err
				}

				log.Info("Successfully updated Service")
			}
		}
	} else {
		// No protocols defined, ensure service is deleted if it exists
		service := &corev1.Service{}
		serviceName := agent.Name
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agent.Namespace}, service)

		if err == nil {
			log.Info("Deleting Service as no protocols are defined")
			if err := r.Delete(ctx, service); err != nil {
				log.Error(err, "Failed to delete Service")
				return ctrl.Result{}, err
			}
		} else if !errors.IsNotFound(err) {
			log.Error(err, "Failed to get Service")
			return ctrl.Result{}, err
		}
	}

	// Check subAgent resolution and update status
	subAgentResolutionErrors := r.resolveSubAgents(ctx, &agent)

	// Update agent status with A2A URL and subAgent resolution status
	if err := r.updateAgentStatus(ctx, &agent, subAgentResolutionErrors); err != nil {
		log.Error(err, "Failed to update agent status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// hasA2AProtocol checks if the agent has A2A protocol configured
func (r *AgentReconciler) hasA2AProtocol(agent *runtimev1alpha1.Agent) bool {
	for _, protocol := range agent.Spec.Protocols {

		if protocol.Type == a2AProtocol {
			return true
		}
	}
	return false
}

// hasOpenAIProtocol checks if the agent has OpenAI protocol configured
func (r *AgentReconciler) hasOpenAIProtocol(agent *runtimev1alpha1.Agent) bool {
	for _, protocol := range agent.Spec.Protocols {
		if protocol.Type == "OpenAI" {
			return true
		}
	}
	return false
}

// getA2AProtocol returns the first A2A protocol configuration found
func (r *AgentReconciler) getA2AProtocol(agent *runtimev1alpha1.Agent) *runtimev1alpha1.AgentProtocol {
	for _, protocol := range agent.Spec.Protocols {
		if protocol.Type == a2AProtocol {
			return &protocol
		}
	}
	return nil
}

// getOpenAIProtocol returns the first OpenAI protocol configuration found
func (r *AgentReconciler) getOpenAIProtocol(agent *runtimev1alpha1.Agent) *runtimev1alpha1.AgentProtocol {
	for _, protocol := range agent.Spec.Protocols {
		if protocol.Type == "OpenAI" {
			return &protocol
		}
	}
	return nil
}

// getProtocolPort returns the port from protocol or default if not specified
func (r *AgentReconciler) getProtocolPort(protocol *runtimev1alpha1.AgentProtocol) int32 {
	defaultPort := int32(8000) // Default port if none specified
	if protocol != nil && protocol.Port != 0 {
		return protocol.Port
	}
	return defaultPort
}

// getA2AHealthPath returns the A2A health check path based on protocol configuration
func (r *AgentReconciler) getA2AHealthPath(protocol *runtimev1alpha1.AgentProtocol) string {
	if protocol != nil && protocol.Path != "" {
		return protocol.Path + agentCardEndpoint
	}
	// Default for agents without protocol specification or path
	return agentCardEndpoint
}

// generateReadinessProbe generates appropriate readiness probe based on agent protocols
func (r *AgentReconciler) generateReadinessProbe(agent *runtimev1alpha1.Agent) *corev1.Probe {
	// Check if agent has external dependencies (subAgents or tools)

	// Priority: A2A > OpenAI > None
	if r.hasA2AProtocol(agent) {
		// Use A2A agent card endpoint for health check
		a2aProtocol := r.getA2AProtocol(agent)
		healthPath := r.getA2AHealthPath(a2aProtocol)
		port := r.getProtocolPort(a2aProtocol)

		probe := r.buildA2AReadinessProbe(healthPath, port)

		return probe
	} else if r.hasOpenAIProtocol(agent) {
		// Use TCP probe for OpenAI-only agents
		openaiProtocol := r.getOpenAIProtocol(agent)
		port := r.getProtocolPort(openaiProtocol)

		probe := r.buildOpenAIReadinessProbe(port)

		return probe
	}

	// No recognized protocols - no readiness probe
	return nil
}

// createDeploymentForAgent creates a deployment for the given Agent
func (r *AgentReconciler) createDeploymentForAgent(ctx context.Context, agent *runtimev1alpha1.Agent, deploymentName string) (*appsv1.Deployment, error) {
	replicas := agent.Spec.Replicas
	if replicas == nil {
		replicas = new(int32)
		*replicas = 1 // Default to 1 replica if not specified
	}

	labels := map[string]string{
		"app":       agent.Name,
		"framework": agent.Spec.Framework,
	}

	// Selector labels should be stable (immutable in Kubernetes)
	selectorLabels := map[string]string{
		"app": agent.Name,
	}

	// Create container ports from protocols
	containerPorts := make([]corev1.ContainerPort, 0, len(agent.Spec.Protocols))
	for _, protocol := range agent.Spec.Protocols {
		port := protocol.Port

		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          protocol.Name,
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Use the image from spec (webhook ensures it's always set)
	agentImage := agent.Spec.Image

	// Build template-specific environment variables
	templateEnvVars, err := r.buildTemplateEnvironmentVars(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to build template environment variables: %w", err)
	}

	// Combine template and user environment variables (user vars override template vars)
	allEnvVars := r.mergeEnvironmentVariables(templateEnvVars, agent.Spec.Env)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:           agentContainerName,
							Image:          agentImage,
							Ports:          containerPorts,
							Env:            allEnvVars,
							EnvFrom:        agent.Spec.EnvFrom,
							ReadinessProbe: r.generateReadinessProbe(agent),
						},
					},
				},
			},
		},
	}

	// Set Agent as the owner of the Deployment
	if err := ctrl.SetControllerReference(agent, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

// createServiceForAgent creates a service for the given Agent
func (r *AgentReconciler) createServiceForAgent(agent *runtimev1alpha1.Agent, serviceName string) (*corev1.Service, error) {
	labels := map[string]string{
		"app":       agent.Name,
		"framework": agent.Spec.Framework,
	}

	// Service selector should match deployment selector (stable labels only)
	selectorLabels := map[string]string{
		"app": agent.Name,
	}

	// Create service ports from protocols
	servicePorts := make([]corev1.ServicePort, 0, len(agent.Spec.Protocols))
	for _, protocol := range agent.Spec.Protocols {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       protocol.Name,
			Port:       protocol.Port,
			TargetPort: intstr.FromInt32(protocol.Port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    servicePorts,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	// Set Agent as the owner of the Service
	if err := ctrl.SetControllerReference(agent, service, r.Scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// needsDeploymentUpdate compares existing and desired deployment
func (r *AgentReconciler) needsDeploymentUpdate(existing, desired *appsv1.Deployment) bool {
	// Check replicas
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// Check labels
	if !r.mapsEqual(existing.Labels, desired.Labels) {
		return true
	}

	if !r.mapsEqual(existing.Spec.Template.Labels, desired.Spec.Template.Labels) {
		return true
	}

	// Check agent container
	existingContainer := r.findAgentContainer(existing.Spec.Template.Spec.Containers)
	desiredContainer := r.findAgentContainer(desired.Spec.Template.Spec.Containers)

	if existingContainer == nil || desiredContainer == nil {
		return existingContainer != desiredContainer
	}

	// Check image
	if existingContainer.Image != desiredContainer.Image {
		return true
	}

	// Check environment variables
	if !equality.EnvVarsEqual(existingContainer.Env, desiredContainer.Env) {
		return true
	}

	// Check environment variable sources
	if !equality.EnvFromEqual(existingContainer.EnvFrom, desiredContainer.EnvFrom) {
		return true
	}

	// Check container ports by name
	if !r.containerPortsEqual(existingContainer.Ports, desiredContainer.Ports) {
		return true
	}

	// Check readiness probes
	if !r.probesEqual(existingContainer.ReadinessProbe, desiredContainer.ReadinessProbe) {
		return true
	}

	return false
}

// needsServiceUpdate compares existing and desired service
func (r *AgentReconciler) needsServiceUpdate(existing, desired *corev1.Service) bool {
	// Check labels
	if !r.mapsEqual(existing.Labels, desired.Labels) {
		return true
	}

	// Check ports by name
	if !r.servicePortsEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}

	return false
}

// updateAgentContainer finds and updates the agent container by name
func (r *AgentReconciler) updateAgentContainer(deployment, desiredDeployment *appsv1.Deployment) error {
	agentContainer := r.findAgentContainer(deployment.Spec.Template.Spec.Containers)
	desiredAgentContainer := r.findAgentContainer(desiredDeployment.Spec.Template.Spec.Containers)

	if agentContainer == nil {
		return fmt.Errorf("agent container not found in existing deployment")
	}
	if desiredAgentContainer == nil {
		return fmt.Errorf("agent container not found in desired deployment")
	}

	// Update container fields
	agentContainer.Image = desiredAgentContainer.Image
	agentContainer.Ports = desiredAgentContainer.Ports
	agentContainer.Env = desiredAgentContainer.Env
	agentContainer.EnvFrom = desiredAgentContainer.EnvFrom
	agentContainer.ReadinessProbe = desiredAgentContainer.ReadinessProbe

	return nil
}

// findAgentContainer finds the agent container by name in a container slice
func (r *AgentReconciler) findAgentContainer(containers []corev1.Container) *corev1.Container {
	for i := range containers {
		if containers[i].Name == agentContainerName {
			return &containers[i]
		}
	}
	// Fallback to first container if no "agent" container found (backwards compatibility)
	if len(containers) > 0 {
		return &containers[0]
	}
	return nil
}

// mapsEqual compares two string maps for equality
func (r *AgentReconciler) mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		if valueB, ok := b[key]; !ok || valueA != valueB {
			return false
		}
	}

	return true
}

// containerPortsEqual compares container ports by name
func (r *AgentReconciler) containerPortsEqual(existing, desired []corev1.ContainerPort) bool {
	if len(existing) != len(desired) {
		return false
	}

	// Create maps for comparison by name
	existingMap := make(map[string]corev1.ContainerPort)
	desiredMap := make(map[string]corev1.ContainerPort)

	for _, port := range existing {
		existingMap[port.Name] = port
	}
	for _, port := range desired {
		desiredMap[port.Name] = port
	}

	if len(existingMap) != len(desiredMap) {
		return false
	}

	for name, existingPort := range existingMap {
		desiredPort, ok := desiredMap[name]
		if !ok {
			return false
		}
		if existingPort.ContainerPort != desiredPort.ContainerPort ||
			existingPort.Protocol != desiredPort.Protocol {
			return false
		}
	}

	return true
}

// servicePortsEqual compares service ports by name
func (r *AgentReconciler) servicePortsEqual(existing, desired []corev1.ServicePort) bool {
	if len(existing) != len(desired) {
		return false
	}

	// Create maps for comparison by name
	existingMap := make(map[string]corev1.ServicePort)
	desiredMap := make(map[string]corev1.ServicePort)

	for _, port := range existing {
		existingMap[port.Name] = port
	}
	for _, port := range desired {
		desiredMap[port.Name] = port
	}

	if len(existingMap) != len(desiredMap) {
		return false
	}

	for name, existingPort := range existingMap {
		desiredPort, ok := desiredMap[name]
		if !ok {
			return false
		}
		if existingPort.Port != desiredPort.Port ||
			existingPort.Protocol != desiredPort.Protocol ||
			existingPort.TargetPort != desiredPort.TargetPort {
			return false
		}
	}

	return true
}

// probesEqual compares two readiness probes for equality
func (r *AgentReconciler) probesEqual(existing, desired *corev1.Probe) bool {
	return equality.ProbesEqual(existing, desired)
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
	namespace := subAgent.AgentRef.Namespace
	if namespace == "" {
		// Default to parent agent's namespace (following Kubernetes conventions)
		namespace = parentNamespace
	}

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
				subAgentNamespace := subAgent.AgentRef.Namespace
				if subAgentNamespace == "" {
					subAgentNamespace = agent.Namespace
				}

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

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(
			&runtimev1alpha1.Agent{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsReferencingSubAgent),
		).
		Named("agent").
		Complete(r)
}

// buildOpenAIReadinessProbe creates TCP-readiness-probe for OpenAI-protocols
func (r *AgentReconciler) buildOpenAIReadinessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
}

// buildA2AReadinessProbe creates HTTP-readiness-probe for A2A-protocols.
func (r *AgentReconciler) buildA2AReadinessProbe(healthPath string, port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: healthPath,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
}
