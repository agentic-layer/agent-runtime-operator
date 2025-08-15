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

			// Update container image and ports (assuming first container is our agent)
			if len(deployment.Spec.Template.Spec.Containers) > 0 && len(desiredDeployment.Spec.Template.Spec.Containers) > 0 {
				deployment.Spec.Template.Spec.Containers[0].Image = desiredDeployment.Spec.Template.Spec.Containers[0].Image
				deployment.Spec.Template.Spec.Containers[0].Ports = desiredDeployment.Spec.Template.Spec.Containers[0].Ports
				deployment.Spec.Template.Spec.Containers[0].Env = desiredDeployment.Spec.Template.Spec.Containers[0].Env
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
							Name:  "agent",
							Image: agent.Spec.Image,
							Ports: containerPorts,
							Env: []corev1.EnvVar{
								{
									Name:  "AGENT_NAME",
									Value: agent.Name,
								},
							},
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

// needsDeploymentUpdate compares existing deployment with desired deployment to determine if update is needed
func (r *AgentReconciler) needsDeploymentUpdate(existing *appsv1.Deployment, desired *appsv1.Deployment) bool {
	// Check replicas
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// Check image
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if existing.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			return true
		}
	}

	// Check labels
	if !mapsEqual(existing.Labels, desired.Labels) {
		return true
	}

	if !mapsEqual(existing.Spec.Template.Labels, desired.Spec.Template.Labels) {
		return true
	}

	// Note: We don't check selector as it's immutable in Kubernetes

	// Check container ports
	existingPorts := existing.Spec.Template.Spec.Containers[0].Ports
	desiredPorts := desired.Spec.Template.Spec.Containers[0].Ports

	if len(existingPorts) != len(desiredPorts) {
		return true
	}

	for i, existingPort := range existingPorts {
		if i >= len(desiredPorts) {
			return true
		}
		desiredPort := desiredPorts[i]
		if existingPort.Name != desiredPort.Name ||
			existingPort.ContainerPort != desiredPort.ContainerPort ||
			existingPort.Protocol != desiredPort.Protocol {
			return true
		}
	}

	return false
}

// needsServiceUpdate compares existing service with desired service to determine if update is needed
func (r *AgentReconciler) needsServiceUpdate(existing *corev1.Service, desired *corev1.Service) bool {
	// Check labels
	if !mapsEqual(existing.Labels, desired.Labels) {
		return true
	}

	// Note: Selector should remain stable, so we don't check for changes

	// Check ports
	existingPorts := existing.Spec.Ports
	desiredPorts := desired.Spec.Ports

	if len(existingPorts) != len(desiredPorts) {
		return true
	}

	for i, existingPort := range existingPorts {
		if i >= len(desiredPorts) {
			return true
		}
		desiredPort := desiredPorts[i]
		if existingPort.Name != desiredPort.Name ||
			existingPort.Port != desiredPort.Port ||
			existingPort.Protocol != desiredPort.Protocol ||
			existingPort.TargetPort != desiredPort.TargetPort {
			return true
		}
	}

	return false
}

// mapsEqual compares two string maps for equality
func mapsEqual(a, b map[string]string) bool {
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

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("agent").
		Complete(r)
}
