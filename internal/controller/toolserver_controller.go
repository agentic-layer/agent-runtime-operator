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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	toolserverContainerName = "toolserver"
	stdioTransport          = "stdio"
	httpTransport           = "http"
	sseTransport            = "sse"
)

// ToolServerReconciler reconciles a ToolServer object
type ToolServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=toolservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=toolservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=toolservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ToolServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the ToolServer instance
	var toolServer runtimev1alpha1.ToolServer
	if err := r.Get(ctx, req.NamespacedName, &toolServer); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ToolServer resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ToolServer")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling ToolServer")

	// Handle stdio transport (sidecar mode - no deployment/service)
	if toolServer.Spec.TransportType == stdioTransport {
		log.Info("ToolServer configured as stdio (sidecar mode), ensuring no deployment/service exists")

		// Clean up any existing deployment/service if switching from http/sse
		if err := r.ensureDeploymentDeleted(ctx, &toolServer); err != nil {
			log.Error(err, "Failed to delete Deployment")
			return ctrl.Result{}, err
		}

		if err := r.ensureServiceDeleted(ctx, &toolServer); err != nil {
			log.Error(err, "Failed to delete Service")
			return ctrl.Result{}, err
		}
	} else {
		// Ensure Deployment exists and is up to date for http/sse transports
		if err := r.ensureDeployment(ctx, &toolServer); err != nil {
			log.Error(err, "Failed to ensure Deployment")
			if statusErr := r.updateToolServerStatusNotReady(ctx, &toolServer, "DeploymentFailed", err.Error()); statusErr != nil {
				log.Error(statusErr, "Failed to update status after deployment failure")
			}
			return ctrl.Result{}, err
		}

		// Ensure Service exists and is up to date for http/sse transports
		if err := r.ensureService(ctx, &toolServer); err != nil {
			log.Error(err, "Failed to ensure Service")
			if statusErr := r.updateToolServerStatusNotReady(ctx, &toolServer, "ServiceFailed", err.Error()); statusErr != nil {
				log.Error(statusErr, "Failed to update status after service failure")
			}
			return ctrl.Result{}, err
		}
	}

	// Update ToolServer status to Ready (optimistic)
	if err := r.updateToolServerStatusReady(ctx, &toolServer); err != nil {
		log.Error(err, "Failed to update ToolServer status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ToolServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.ToolServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("toolserver").
		Complete(r)
}

// ensureDeployment ensures the Deployment for the ToolServer exists and is up to date
func (r *ToolServerReconciler) ensureDeployment(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) error {
	log := logf.FromContext(ctx)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      toolServer.Name,
			Namespace: toolServer.Namespace,
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
			"app":       toolServer.Name,
			"protocol":  toolServer.Spec.Protocol,
			"transport": toolServer.Spec.TransportType,
		}

		// Selector labels (immutable)
		selectorLabels := map[string]string{
			"app": toolServer.Name,
		}

		// Build container ports
		containerPorts := []corev1.ContainerPort{
			{
				Name:          "toolserver",
				ContainerPort: toolServer.Spec.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		}

		// Set immutable fields only on creation
		if deployment.CreationTimestamp.IsZero() {
			// Deployment selector is immutable so we set this value only if
			// a new object is going to be created
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			}
		}

		// Set selector labels for the pod template
		deployment.Spec.Template.Labels = selectorLabels

		// Set/update replicas
		if deployment.Spec.Replicas == nil {
			deployment.Spec.Replicas = new(int32)
		}
		if toolServer.Spec.Replicas != nil {
			*deployment.Spec.Replicas = *toolServer.Spec.Replicas
		} else {
			*deployment.Spec.Replicas = 1
		}

		// Merge managed labels (preserving unmanaged labels)
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		maps.Copy(deployment.Labels, managedLabels)

		// Update toolserver container fields
		container := findContainerByName(&deployment.Spec.Template.Spec, toolserverContainerName)
		if container == nil {
			// Container doesn't exist, create and append it
			newContainer := corev1.Container{
				Name: toolserverContainerName,
			}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, newContainer)
			// Get pointer to the newly added container
			container = &deployment.Spec.Template.Spec.Containers[len(deployment.Spec.Template.Spec.Containers)-1]
		}
		container.Image = toolServer.Spec.Image
		container.Command = toolServer.Spec.Command
		container.Args = toolServer.Spec.Args
		container.Ports = containerPorts
		container.Env = toolServer.Spec.Env
		container.EnvFrom = toolServer.Spec.EnvFrom
		container.ReadinessProbe = r.buildReadinessProbe(toolServer.Spec.Port)

		// Set owner reference
		return ctrl.SetControllerReference(toolServer, deployment, r.Scheme)
	}); err != nil {
		return err
	} else if op != controllerutil.OperationResultNone {
		log.Info("Deployment reconciled", "operation", op)
	}

	return nil
}

// ensureService ensures the Service for the ToolServer exists and is up to date
func (r *ToolServerReconciler) ensureService(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) error {
	log := logf.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      toolServer.Name,
			Namespace: toolServer.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Build managed labels
		managedLabels := map[string]string{
			"app":       toolServer.Name,
			"protocol":  toolServer.Spec.Protocol,
			"transport": toolServer.Spec.TransportType,
		}

		// Service selector (stable labels only)
		selectorLabels := map[string]string{
			"app": toolServer.Name,
		}

		// Build service ports
		servicePorts := []corev1.ServicePort{
			{
				Name:       "toolserver",
				Port:       toolServer.Spec.Port,
				TargetPort: intstr.FromInt32(toolServer.Spec.Port),
				Protocol:   corev1.ProtocolTCP,
			},
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
		return ctrl.SetControllerReference(toolServer, service, r.Scheme)
	}); err != nil {
		return err
	} else if op != controllerutil.OperationResultNone {
		log.Info("Service reconciled", "operation", op)
	}

	return nil
}

// buildReadinessProbe creates a TCP readiness probe for the tool server
func (r *ToolServerReconciler) buildReadinessProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(port)),
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		SuccessThreshold:    1,
		FailureThreshold:    10,
	}
}

// ensureDeploymentDeleted ensures the Deployment is deleted if it exists
func (r *ToolServerReconciler) ensureDeploymentDeleted(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) error {
	log := logf.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: toolServer.Name, Namespace: toolServer.Namespace}, deployment); err == nil {
		log.Info("Deleting Deployment as transport is stdio (sidecar mode)")
		if err := r.Delete(ctx, deployment); err != nil {
			return fmt.Errorf("failed to delete Deployment: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}
	return nil
}

// ensureServiceDeleted ensures the Service is deleted if it exists
func (r *ToolServerReconciler) ensureServiceDeleted(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) error {
	log := logf.FromContext(ctx)

	service := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: toolServer.Name, Namespace: toolServer.Namespace}, service); err == nil {
		log.Info("Deleting Service as transport is stdio (sidecar mode)")
		if err := r.Delete(ctx, service); err != nil {
			return fmt.Errorf("failed to delete Service: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get Service: %w", err)
	}
	return nil
}

// updateToolServerStatusReady sets the ToolServer status to Ready and updates the URL
func (r *ToolServerReconciler) updateToolServerStatusReady(ctx context.Context, toolServer *runtimev1alpha1.ToolServer) error {
	// Build URL for http/sse transports
	if toolServer.Spec.TransportType == httpTransport || toolServer.Spec.TransportType == sseTransport {
		toolServer.Status.Url = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s",
			toolServer.Name, toolServer.Namespace, toolServer.Spec.Port, toolServer.Spec.Path)
	} else {
		toolServer.Status.Url = ""
	}

	// Set Ready condition to True
	meta.SetStatusCondition(&toolServer.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "ToolServer is ready",
		ObservedGeneration: toolServer.Generation,
	})

	if err := r.Status().Update(ctx, toolServer); err != nil {
		return fmt.Errorf("failed to update toolserver status: %w", err)
	}

	return nil
}

// updateToolServerStatusNotReady sets the ToolServer status to not Ready
func (r *ToolServerReconciler) updateToolServerStatusNotReady(ctx context.Context, toolServer *runtimev1alpha1.ToolServer, reason, message string) error {
	// Set Ready condition to False
	meta.SetStatusCondition(&toolServer.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: toolServer.Generation,
	})

	if err := r.Status().Update(ctx, toolServer); err != nil {
		return fmt.Errorf("failed to update toolserver status: %w", err)
	}

	return nil
}
