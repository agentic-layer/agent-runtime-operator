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
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/equality"
)

const (
	agentContainerName = "agent"
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

	log.Info("Reconciling Agent", "name", agent.Name, "namespace", agent.Namespace)

	// Check if deployment already exists
	deployment := &appsv1.Deployment{}
	deploymentName := agent.Name
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agent.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		// Deployment does not exist, create it
		log.Info("Creating new Deployment", "name", deploymentName, "namespace", agent.Namespace)

		deployment, err = r.createDeploymentForAgent(&agent, deploymentName)
		if err != nil {
			log.Error(err, "Failed to create deployment for Agent")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create new Deployment", "name", deploymentName)
			return ctrl.Result{}, err
		}

		log.Info("Successfully created Deployment", "name", deploymentName, "namespace", agent.Namespace)
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	} else {
		// Deployment exists, check if it needs to be updated
		desiredDeployment, err := r.createDeploymentForAgent(&agent, deploymentName)
		if err != nil {
			log.Error(err, "Failed to create desired deployment spec for Agent")
			return ctrl.Result{}, err
		}

		if r.needsDeploymentUpdate(deployment, desiredDeployment) {
			log.Info("Updating existing Deployment", "name", deploymentName, "namespace", agent.Namespace)

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
				log.Error(err, "Failed to update Deployment", "name", deploymentName)
				return ctrl.Result{}, err
			}

			log.Info("Successfully updated Deployment", "name", deploymentName, "namespace", agent.Namespace)
		}
	}

	// Create service if protocols are defined
	if len(agent.Spec.Protocols) > 0 {
		service := &corev1.Service{}
		serviceName := agent.Name
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agent.Namespace}, service)

		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating new Service", "name", serviceName, "namespace", agent.Namespace)

			service, err = r.createServiceForAgent(&agent, serviceName)
			if err != nil {
				log.Error(err, "Failed to create service for Agent")
				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create new Service", "name", serviceName)
				return ctrl.Result{}, err
			}

			log.Info("Successfully created Service", "name", serviceName, "namespace", agent.Namespace)
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
				log.Info("Updating existing Service", "name", serviceName, "namespace", agent.Namespace)

				// Update service spec
				// Note: Selector should remain stable, only update ports and labels
				service.Spec.Ports = desiredService.Spec.Ports
				service.Labels = desiredService.Labels

				if err := r.Update(ctx, service); err != nil {
					log.Error(err, "Failed to update Service", "name", serviceName)
					return ctrl.Result{}, err
				}

				log.Info("Successfully updated Service", "name", serviceName, "namespace", agent.Namespace)
			}
		}
	} else {
		// No protocols defined, ensure service is deleted if it exists
		service := &corev1.Service{}
		serviceName := agent.Name
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agent.Namespace}, service)

		if err == nil {
			log.Info("Deleting Service as no protocols are defined", "name", serviceName, "namespace", agent.Namespace)
			if err := r.Delete(ctx, service); err != nil {
				log.Error(err, "Failed to delete Service", "name", serviceName)
				return ctrl.Result{}, err
			}
		} else if !errors.IsNotFound(err) {
			log.Error(err, "Failed to get Service")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildTemplateEnvironmentVars creates template environment variables from Agent spec fields.
// These template variables are always set regardless of whether using template or custom images,
// providing a consistent interface for agent configuration.
//
// Template variables created:
//   - AGENT_NAME: Always set to the agent's name
//   - AGENT_DESCRIPTION: Set to spec.Description (empty string if not specified)
//   - AGENT_INSTRUCTION: Set to spec.Instruction (empty string if not specified)
//   - AGENT_MODEL: Set to spec.Model (empty string if not specified)
//   - SUB_AGENTS: JSON-encoded map of sub-agent configurations (empty object if none)
//   - AGENT_TOOLS: JSON-encoded map of MCP tool configurations (empty object if none)
//
// JSON Structure:
//   - SubAgents: {"agentName": {"url": "https://..."}}
//   - Tools: {"toolName": {"url": "https://..."}}
//
// Parameters:
//   - agent: The Agent resource to generate template variables for
//
// Returns:
//   - []corev1.EnvVar: Slice of environment variables for template configuration
//   - error: JSON marshaling error if SubAgents or Tools contain invalid data
func (r *AgentReconciler) buildTemplateEnvironmentVars(agent *runtimev1alpha1.Agent) ([]corev1.EnvVar, error) {
	var templateEnvVars []corev1.EnvVar

	// Always set AGENT_NAME (sanitized to meet environment variable requirements)
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_NAME",
		Value: r.sanitizeAgentName(agent.Name),
	})

	// AGENT_DESCRIPTION - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_DESCRIPTION",
		Value: agent.Spec.Description,
	})

	// AGENT_INSTRUCTION - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_INSTRUCTION",
		Value: agent.Spec.Instruction,
	})

	// AGENT_MODEL - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_MODEL",
		Value: agent.Spec.Model,
	})

	// SUB_AGENTS - always set, with empty object if no subagents
	var subAgentsJSON []byte
	var err error
	if len(agent.Spec.SubAgents) > 0 {
		subAgentsMap := make(map[string]map[string]string)
		for _, subAgent := range agent.Spec.SubAgents {
			subAgentsMap[subAgent.Name] = map[string]string{
				"url": subAgent.Url,
			}
		}
		subAgentsJSON, err = json.Marshal(subAgentsMap)
	} else {
		subAgentsJSON, err = json.Marshal(map[string]interface{}{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subAgents: %w", err)
	}
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "SUB_AGENTS",
		Value: string(subAgentsJSON),
	})

	// AGENT_TOOLS - always set, with empty object if no tools
	var toolsJSON []byte
	if len(agent.Spec.Tools) > 0 {
		toolsMap := make(map[string]map[string]string)
		for _, tool := range agent.Spec.Tools {
			toolsMap[tool.Name] = map[string]string{
				"url": tool.Url,
			}
		}
		toolsJSON, err = json.Marshal(toolsMap)
	} else {
		toolsJSON, err = json.Marshal(map[string]interface{}{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tools: %w", err)
	}
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_TOOLS",
		Value: string(toolsJSON),
	})

	return templateEnvVars, nil
}

// mergeEnvironmentVariables merges template and user environment variables with proper precedence.
// User-defined environment variables override template variables with the same name, ensuring
// that users can customize agent behavior while maintaining template functionality.
//
// Merge Logic:
//  1. Start with all template variables
//  2. For each template variable, check if user has provided an override
//  3. If user override exists, use the user value instead of template value
//  4. Add any additional user variables that don't override template variables
//  5. Maintain original ordering where possible
//
// Example:
//
//	Template: [AGENT_NAME=test, AGENT_MODEL=default, TEMPLATE_VAR=value]
//	User:     [AGENT_MODEL=custom, USER_VAR=user]
//	Result:   [AGENT_NAME=test, AGENT_MODEL=custom, TEMPLATE_VAR=value, USER_VAR=user]
//
// Parameters:
//   - templateEnvVars: Environment variables generated from Agent template fields
//   - userEnvVars: Environment variables defined by user in Agent.Spec.Env
//
// Returns:
//   - []corev1.EnvVar: Merged environment variables with user precedence
func (r *AgentReconciler) mergeEnvironmentVariables(templateEnvVars, userEnvVars []corev1.EnvVar) []corev1.EnvVar {
	// Create a map for efficient lookups of user environment variables
	userEnvMap := make(map[string]corev1.EnvVar)
	for _, env := range userEnvVars {
		userEnvMap[env.Name] = env
	}

	// Pre-allocate result slice for efficiency
	result := make([]corev1.EnvVar, 0, len(templateEnvVars)+len(userEnvVars))

	// Add template environment variables, but skip if user has overridden them
	for _, templateEnv := range templateEnvVars {
		if userEnv, exists := userEnvMap[templateEnv.Name]; exists {
			// User has overridden this variable, use user's version
			result = append(result, userEnv)
			delete(userEnvMap, templateEnv.Name) // Remove so we don't add it again
		} else {
			// No user override, use template variable
			result = append(result, templateEnv)
		}
	}

	// Add any remaining user environment variables that weren't overrides
	for _, userEnv := range userEnvMap {
		result = append(result, userEnv)
	}

	return result
}

// createDeploymentForAgent creates a deployment for the given Agent
func (r *AgentReconciler) createDeploymentForAgent(agent *runtimev1alpha1.Agent, deploymentName string) (*appsv1.Deployment, error) {
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
	templateEnvVars, err := r.buildTemplateEnvironmentVars(agent)
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
							Name:    agentContainerName,
							Image:   agentImage,
							Ports:   containerPorts,
							Env:     allEnvVars,
							EnvFrom: agent.Spec.EnvFrom,
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

// needsDeploymentUpdate compares existing and desired deployment (addresses PR feedback)
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

	// Check environment variables (addresses PR feedback)
	if !equality.EnvVarsEqual(existingContainer.Env, desiredContainer.Env) {
		return true
	}

	// Check environment variable sources
	if !equality.EnvFromEqual(existingContainer.EnvFrom, desiredContainer.EnvFrom) {
		return true
	}

	// Check container ports by name (addresses PR feedback)
	if !r.containerPortsEqual(existingContainer.Ports, desiredContainer.Ports) {
		return true
	}

	return false
}

// needsServiceUpdate compares existing and desired service (addresses PR feedback)
func (r *AgentReconciler) needsServiceUpdate(existing, desired *corev1.Service) bool {
	// Check labels
	if !r.mapsEqual(existing.Labels, desired.Labels) {
		return true
	}

	// Check ports by name (addresses PR feedback)
	if !r.servicePortsEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}

	return false
}

// updateAgentContainer finds and updates the agent container by name (addresses PR feedback)
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

// containerPortsEqual compares container ports by name (addresses PR feedback)
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

// servicePortsEqual compares service ports by name (addresses PR feedback)
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

// sanitizeAgentName sanitizes the agent name to meet environment variable naming requirements.
// Environment variable names should start with a letter (a-z, A-Z) or underscore (_),
// and can only contain letters, digits (0-9), and underscores.
func (r *AgentReconciler) sanitizeAgentName(name string) string {
	var result strings.Builder

	// Process each character
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_':
			// Valid character, keep it
			result.WriteRune(r)
		case r == '-':
			// Convert hyphens to underscores
			result.WriteRune('_')
		default:
			// Replace any other character with underscore
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// Ensure it starts with a letter or underscore
	if len(sanitized) > 0 {
		firstChar := sanitized[0]
		if firstChar >= '0' && firstChar <= '9' {
			// Starts with digit, prepend underscore
			sanitized = "_" + sanitized
		}
	}

	// Ensure we have a valid result
	if sanitized == "" {
		sanitized = agentContainerName
	}

	return sanitized
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("agent").
		Complete(r)
}
