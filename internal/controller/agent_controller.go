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
	"maps"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	agentContainerName           = "agent"
	agentCardEndpoint            = "/.well-known/agent-card.json"
	googleAdkFramework           = "google-adk"
	DefaultTemplateImageAdk      = "ghcr.io/agentic-layer/agent-template-adk:0.5.1"
	defaultTemplateImageFallback = "invalid"
)

// AgentReconciler reconciles a Agent object
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=toolservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=toolservers/status,verbs=get
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=aigateways,verbs=get;list;watch
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

	log.V(1).Info("Reconciling Agent")

	// Resolve subAgents early - fail fast if any cannot be resolved
	resolvedSubAgents, err := r.resolveAllSubAgents(ctx, &agent)
	if err != nil {
		log.Error(err, "Failed to resolve subAgents")
		return ctrl.Result{}, err
	}

	// Resolve Tools from ToolServer references early - fail fast if any cannot be resolved
	resolvedTools, err := r.resolveAllTools(ctx, &agent)
	if err != nil {
		log.Error(err, "Failed to resolve tools")
		return ctrl.Result{}, err
	}

	// Resolve AiGateway (optional - returns nil if not found)
	aiGateway, err := r.resolveAiGateway(ctx, &agent)
	if err != nil {
		log.Error(err, "Failed to resolve AiGateway")
		// Update status to reflect missing AiGateway
		if statusErr := r.updateAgentStatusNotReady(ctx, &agent, "MissingAiGateway", err.Error()); statusErr != nil {
			log.Error(statusErr, "Failed to update status after AiGateway resolution failure")
		}
		return ctrl.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.ensureDeployment(ctx, &agent, resolvedSubAgents, resolvedTools, aiGateway)
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.ensureService(ctx, &agent)
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.updateAgentStatusReady(ctx, &agent, aiGateway)
	}); err != nil {
		return ctrl.Result{}, err
	}

	log.V(1).Info("Reconciled Agent")

	return ctrl.Result{}, nil
}

// ensureDeployment ensures the Deployment for the Agent exists and is up to date
func (r *AgentReconciler) ensureDeployment(ctx context.Context, agent *runtimev1alpha1.Agent,
	resolvedSubAgents map[string]string, resolvedTools map[string]string, aiGateway *runtimev1alpha1.AiGateway) error {
	log := logf.FromContext(ctx)

	log.V(1).Info("Ensuring Deployment for Agent")

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
		},
	}

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Build managed labels
		managedLabels := map[string]string{
			"app":       agent.Name,
			"framework": agent.Spec.Framework,
		}

		// Selector labels (immutable)
		selectorLabels := map[string]string{
			"app": agent.Name,
		}

		// Build container ports from protocols
		containerPorts := make([]corev1.ContainerPort, 0, len(agent.Spec.Protocols))
		for _, protocol := range agent.Spec.Protocols {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				Name:          protocol.Name,
				ContainerPort: protocol.Port,
				Protocol:      corev1.ProtocolTCP,
			})
		}

		// Build template environment variables
		var aiGatewayUrl *string
		if aiGateway != nil {
			url := r.buildAiGatewayServiceUrl(*aiGateway)
			aiGatewayUrl = &url
		}
		templateEnvVars, err := r.buildTemplateEnvironmentVars(agent, resolvedSubAgents, resolvedTools, aiGatewayUrl)
		if err != nil {
			return fmt.Errorf("failed to build template environment variables: %w", err)
		}

		// Merge template and user environment variables
		allEnvVars := r.mergeEnvironmentVariables(templateEnvVars, agent.Spec.Env)

		// Set immutable fields only on creation
		if deployment.CreationTimestamp.IsZero() {
			// Deployment selector is immutable so we set this value only if
			// a new object is going to be created
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			}
		}

		// Set selector labels for the pod template respectively
		deployment.Spec.Template.Labels = selectorLabels

		// Set replicas
		if deployment.Spec.Replicas == nil {
			deployment.Spec.Replicas = new(int32)
		}
		if agent.Spec.Replicas != nil {
			*deployment.Spec.Replicas = *agent.Spec.Replicas
		}

		// Merge managed labels (preserving unmanaged labels)
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		maps.Copy(deployment.Labels, managedLabels)

		// Update agent container fields
		container := findContainerByName(&deployment.Spec.Template.Spec, agentContainerName)
		if container == nil {
			// Container doesn't exist, create and append it
			newContainer := corev1.Container{
				Name: agentContainerName,
			}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, newContainer)
			// Get pointer to the newly added container
			container = &deployment.Spec.Template.Spec.Containers[len(deployment.Spec.Template.Spec.Containers)-1]
		}
		if agent.Spec.Image != "" {
			container.Image = agent.Spec.Image
		} else {
			switch agent.Spec.Framework {
			case googleAdkFramework:
				container.Image = DefaultTemplateImageAdk
			default:
				// Validation will catch unsupported frameworks without images
				// This shouldn't be reached due to validation, but set template as fallback
				container.Image = defaultTemplateImageFallback
			}
		}
		container.Ports = containerPorts
		container.Env = allEnvVars
		container.EnvFrom = agent.Spec.EnvFrom
		container.VolumeMounts = agent.Spec.VolumeMounts
		container.ReadinessProbe = r.generateReadinessProbe(agent)

		// Update pod volumes
		deployment.Spec.Template.Spec.Volumes = agent.Spec.Volumes

		// Set owner reference
		return ctrl.SetControllerReference(agent, deployment, r.Scheme)
	}); err != nil {
		return err
	} else if op != controllerutil.OperationResultNone {
		log.Info("Deployment reconciled", "operation", op, "obj", *deployment)
	} else {
		log.V(1).Info("Deployment up to date")
	}

	return nil
}

// ensureService ensures the Service for the Agent exists and is up to date, or is deleted if no protocols are defined
func (r *AgentReconciler) ensureService(ctx context.Context, agent *runtimev1alpha1.Agent) error {
	log := logf.FromContext(ctx)

	log.V(1).Info("Ensuring Service for Agent")

	if len(agent.Spec.Protocols) == 0 {
		// No protocols defined, ensure service is deleted if it exists
		service := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}, service); err == nil {
			log.Info("Deleting Service as no protocols are defined")
			if err := r.Delete(ctx, service); err != nil {
				return fmt.Errorf("failed to delete Service: %w", err)
			}
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Service: %w", err)
		}
		return nil
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Build managed labels
		managedLabels := map[string]string{
			"app":       agent.Name,
			"framework": agent.Spec.Framework,
		}

		// Service selector (stable labels only)
		selectorLabels := map[string]string{
			"app": agent.Name,
		}

		// Build service ports from protocols
		servicePorts := make([]corev1.ServicePort, 0, len(agent.Spec.Protocols))
		for _, protocol := range agent.Spec.Protocols {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       protocol.Name,
				Port:       protocol.Port,
				TargetPort: intstr.FromInt32(protocol.Port),
				Protocol:   corev1.ProtocolTCP,
			})
		}

		// Merge managed labels (preserving unmanaged labels)
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}
		maps.Copy(service.Labels, managedLabels)

		// Update service spec
		service.Spec.Ports = servicePorts
		service.Spec.Selector = selectorLabels
		service.Spec.Type = corev1.ServiceTypeClusterIP

		// Set owner reference
		return ctrl.SetControllerReference(agent, service, r.Scheme)
	}); err != nil {
		return err
	} else if op != controllerutil.OperationResultNone {
		log.Info("Service reconciled", "operation", op, "obj", *service)
	} else {
		log.V(1).Info("Service up to date")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log := logf.FromContext(context.Background())

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(
			&runtimev1alpha1.Agent{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsReferencingSubAgent),
		).
		Watches(
			&runtimev1alpha1.ToolServer{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsReferencingToolServer),
		)

	// Only watch AiGateway if the CRD is installed
	if isAiGatewayCRDInstalled(mgr) {
		log.Info("AiGateway CRD detected, enabling watch")
		builder = builder.Watches(
			&runtimev1alpha1.AiGateway{},
			handler.EnqueueRequestsFromMapFunc(r.findAgentsReferencingAiGateway),
		)
	} else {
		log.Info("AiGateway CRD not installed, skipping watch (agent will work without AI Gateway integration)")
	}

	return builder.Named("agent").Complete(r)
}

// isAiGatewayCRDInstalled checks if the AiGateway CRD is installed in the cluster
func isAiGatewayCRDInstalled(mgr ctrl.Manager) bool {
	gvk := runtimev1alpha1.GroupVersion.WithKind("AiGateway")
	_, err := mgr.GetRESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	return err == nil
}
