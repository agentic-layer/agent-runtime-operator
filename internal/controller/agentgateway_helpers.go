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
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// reconcileDeployment creates or updates the gateway Deployment
func (r *AgentGatewayReconciler) reconcileDeployment(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, configHash string) error {
	log := logf.FromContext(ctx)

	deployment := r.createDeploymentForGateway(gateway, configHash)

	// Check if Deployment exists
	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, existing)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Deployment", "name", deployment.Name)
		return r.Create(ctx, deployment)
	} else if err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	// Update if needed (check config hash annotation)
	existingHash := existing.Spec.Template.Annotations["gateway.agentic-layer.ai/config-hash"]
	if existingHash != configHash || r.needsDeploymentUpdate(existing, deployment) {
		log.Info("Updating Deployment", "name", deployment.Name)
		existing.Spec = deployment.Spec
		existing.Labels = deployment.Labels
		existing.Annotations = deployment.Annotations
		return r.Update(ctx, existing)
	}

	return nil
}

// createDeploymentForGateway creates a Deployment spec for the gateway
func (r *AgentGatewayReconciler) createDeploymentForGateway(gateway *runtimev1alpha1.AgentGateway, configHash string) *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name":       "agentgateway",
		"app.kubernetes.io/instance":   gateway.Name,
		"app.kubernetes.io/managed-by": "agent-runtime-operator",
		"app.kubernetes.io/component":  "gateway",
	}

	// Add custom labels
	if gateway.Spec.Labels != nil {
		for k, v := range gateway.Spec.Labels {
			labels[k] = v
		}
	}

	annotations := map[string]string{
		"gateway.agentic-layer.ai/config-hash": configHash,
	}

	// Add custom annotations
	if gateway.Spec.Annotations != nil {
		for k, v := range gateway.Spec.Annotations {
			annotations[k] = v
		}
	}

	// Add IAP annotations if enabled
	if gateway.Spec.IAP.Enabled {
		annotations["kubernetes.io/ingress.class"] = "gce"
		annotations["cloud.google.com/load-balancer-type"] = "External"
		if gateway.Spec.IAP.ClientID != "" {
			annotations["ingress.gcp.kubernetes.io/backends"] = fmt.Sprintf(`{"default":{"iap":{"enabled":true,"oauthclientCredentials":{"clientID":"%s"}}}}`, gateway.Spec.IAP.ClientID)
		}
	}

	replicas := int32(2)
	if gateway.Spec.Gateway.Replicas != nil {
		replicas = *gateway.Spec.Gateway.Replicas
	}

	image := "devopsfaith/krakend:2.7"
	if gateway.Spec.Gateway.Image != "" {
		image = gateway.Spec.Gateway.Image
	}

	// Resource requirements
	resources := corev1.ResourceRequirements{}
	if gateway.Spec.Gateway.Resources != nil {
		if gateway.Spec.Gateway.Resources.Requests != nil {
			resources.Requests = corev1.ResourceList{}
			for k, v := range gateway.Spec.Gateway.Resources.Requests {
				resources.Requests[corev1.ResourceName(k)] = resource.MustParse(v)
			}
		}
		if gateway.Spec.Gateway.Resources.Limits != nil {
			resources.Limits = corev1.ResourceList{}
			for k, v := range gateway.Spec.Gateway.Resources.Limits {
				resources.Limits[corev1.ResourceName(k)] = resource.MustParse(v)
			}
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-gateway", gateway.Name),
			Namespace:   gateway.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "agentgateway",
					"app.kubernetes.io/instance": gateway.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"gateway.agentic-layer.ai/config-hash": configHash,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "krakend",
							Image: image,
							Args: []string{
								"run",
								"-c",
								"/etc/krakend/krakend.json",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/krakend",
									ReadOnly:  true,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/__health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/__health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: fmt.Sprintf("%s-config", gateway.Name),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(gateway, deployment, r.Scheme)

	return deployment
}

// needsDeploymentUpdate checks if a deployment needs updating
func (r *AgentGatewayReconciler) needsDeploymentUpdate(existing, desired *appsv1.Deployment) bool {
	// Check if replicas differ
	if existing.Spec.Replicas == nil || desired.Spec.Replicas == nil {
		return existing.Spec.Replicas != desired.Spec.Replicas
	}
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// Check container image
	if len(existing.Spec.Template.Spec.Containers) == 0 || len(desired.Spec.Template.Spec.Containers) == 0 {
		return true
	}

	existingImage := existing.Spec.Template.Spec.Containers[0].Image
	desiredImage := desired.Spec.Template.Spec.Containers[0].Image

	return existingImage != desiredImage
}

// reconcileService creates or updates the gateway Service
func (r *AgentGatewayReconciler) reconcileService(ctx context.Context, gateway *runtimev1alpha1.AgentGateway) error {
	log := logf.FromContext(ctx)

	service := r.createServiceForGateway(gateway)

	// Check if Service exists
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, existing)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Service", "name", service.Name)
		return r.Create(ctx, service)
	} else if err != nil {
		return fmt.Errorf("failed to get Service: %w", err)
	}

	// Update if needed
	if r.needsServiceUpdate(existing, service) {
		log.Info("Updating Service", "name", service.Name)
		existing.Spec.Ports = service.Spec.Ports
		existing.Spec.Selector = service.Spec.Selector
		existing.Labels = service.Labels
		existing.Annotations = service.Annotations
		return r.Update(ctx, existing)
	}

	return nil
}

// createServiceForGateway creates a Service spec for the gateway
func (r *AgentGatewayReconciler) createServiceForGateway(gateway *runtimev1alpha1.AgentGateway) *corev1.Service {
	labels := map[string]string{
		"app.kubernetes.io/name":       "agentgateway",
		"app.kubernetes.io/instance":   gateway.Name,
		"app.kubernetes.io/managed-by": "agent-runtime-operator",
		"app.kubernetes.io/component":  "gateway",
	}

	// Add custom labels
	if gateway.Spec.Labels != nil {
		for k, v := range gateway.Spec.Labels {
			labels[k] = v
		}
	}

	annotations := map[string]string{}
	if gateway.Spec.Annotations != nil {
		for k, v := range gateway.Spec.Annotations {
			annotations[k] = v
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-gateway", gateway.Name),
			Namespace:   gateway.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":     "agentgateway",
				"app.kubernetes.io/instance": gateway.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(gateway, service, r.Scheme)

	return service
}

// needsServiceUpdate checks if a service needs updating
func (r *AgentGatewayReconciler) needsServiceUpdate(existing, desired *corev1.Service) bool {
	// Compare ports
	if len(existing.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}

	for i, existingPort := range existing.Spec.Ports {
		if i >= len(desired.Spec.Ports) {
			return true
		}
		desiredPort := desired.Spec.Ports[i]
		if existingPort.Name != desiredPort.Name ||
			existingPort.Port != desiredPort.Port ||
			existingPort.TargetPort != desiredPort.TargetPort {
			return true
		}
	}

	return false
}

// reconcileIngress creates or updates the gateway Ingress
func (r *AgentGatewayReconciler) reconcileIngress(ctx context.Context, gateway *runtimev1alpha1.AgentGateway) error {
	log := logf.FromContext(ctx)

	ingress := r.createIngressForGateway(gateway)

	// Check if Ingress exists
	existing := &netv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, existing)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Ingress", "name", ingress.Name)
		return r.Create(ctx, ingress)
	} else if err != nil {
		return fmt.Errorf("failed to get Ingress: %w", err)
	}

	// Update if needed
	log.Info("Updating Ingress", "name", ingress.Name)
	existing.Spec = ingress.Spec
	existing.Labels = ingress.Labels
	existing.Annotations = ingress.Annotations
	return r.Update(ctx, existing)
}

// createIngressForGateway creates an Ingress spec for the gateway
func (r *AgentGatewayReconciler) createIngressForGateway(gateway *runtimev1alpha1.AgentGateway) *netv1.Ingress {
	labels := map[string]string{
		"app.kubernetes.io/name":       "agentgateway",
		"app.kubernetes.io/instance":   gateway.Name,
		"app.kubernetes.io/managed-by": "agent-runtime-operator",
		"app.kubernetes.io/component":  "ingress",
	}

	// Add custom labels
	if gateway.Spec.Labels != nil {
		for k, v := range gateway.Spec.Labels {
			labels[k] = v
		}
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class": "nginx",
	}

	// Add IAP annotations if enabled
	if gateway.Spec.IAP.Enabled {
		annotations["kubernetes.io/ingress.class"] = "gce"
		annotations["kubernetes.io/ingress.global-static-ip-name"] = fmt.Sprintf("%s-ip", gateway.Name)
		annotations["ingress.gcp.kubernetes.io/managed-certificates"] = fmt.Sprintf("%s-ssl-cert", gateway.Name)

		if gateway.Spec.IAP.ClientID != "" {
			iapConfig := fmt.Sprintf(`{"default":{"iap":{"enabled":true,"oauthclientCredentials":{"clientID":"%s"}}}}`, gateway.Spec.IAP.ClientID)
			annotations["ingress.gcp.kubernetes.io/backends"] = iapConfig
		}
	}

	// Add custom annotations
	if gateway.Spec.Annotations != nil {
		for k, v := range gateway.Spec.Annotations {
			annotations[k] = v
		}
	}

	serviceName := fmt.Sprintf("%s-gateway", gateway.Name)
	pathType := netv1.PathTypePrefix

	// Build rules based on routing strategy
	var rules []netv1.IngressRule

	switch gateway.Spec.RoutingStrategy {
	case runtimev1alpha1.SubdomainBasedRouting:
		// Create rules for each subdomain
		for _, agentRef := range gateway.Spec.Agents {
			if !agentRef.Enabled {
				continue
			}

			host := fmt.Sprintf("%s.%s", agentRef.RoutePrefix, gateway.Spec.Domain)
			rule := netv1.IngressRule{
				Host: host,
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: netv1.IngressBackend{
									Service: &netv1.IngressServiceBackend{
										Name: serviceName,
										Port: netv1.ServiceBackendPort{
											Number: 80,
										},
									},
								},
							},
						},
					},
				},
			}
			rules = append(rules, rule)
		}
	default: // Path-based routing
		rule := netv1.IngressRule{
			Host: gateway.Spec.Domain,
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: serviceName,
									Port: netv1.ServiceBackendPort{
										Number: 80,
									},
								},
							},
						},
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	// TLS configuration
	var tls []netv1.IngressTLS
	if gateway.Spec.TLS.Enabled && gateway.Spec.TLS.SecretName != "" {
		tls = append(tls, netv1.IngressTLS{
			Hosts:      gateway.Spec.TLS.Hosts,
			SecretName: gateway.Spec.TLS.SecretName,
		})
	}

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-gateway", gateway.Name),
			Namespace:   gateway.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: rules,
			TLS:   tls,
		},
	}

	// Set owner reference
	controllerutil.SetControllerReference(gateway, ingress, r.Scheme)

	return ingress
}

// updateCondition updates or adds a condition to the AgentGateway status
func (r *AgentGatewayReconciler) updateCondition(gateway *runtimev1alpha1.AgentGateway, conditionType runtimev1alpha1.GatewayConditionType, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Find existing condition
	for i, existingCondition := range gateway.Status.Conditions {
		if existingCondition.Type == condition.Type {
			// Update existing condition if status changed
			if existingCondition.Status != condition.Status {
				gateway.Status.Conditions[i] = condition
			} else {
				// Update message and reason but keep transition time
				gateway.Status.Conditions[i].Message = condition.Message
				gateway.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}

	// Add new condition
	gateway.Status.Conditions = append(gateway.Status.Conditions, condition)
}

// updateStatus updates the AgentGateway status
func (r *AgentGatewayReconciler) updateStatus(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, result ctrl.Result, err error) (ctrl.Result, error) {
	if statusErr := r.Status().Update(ctx, gateway); statusErr != nil {
		log := logf.FromContext(ctx)
		log.Error(statusErr, "Failed to update AgentGateway status")
		return ctrl.Result{}, statusErr
	}
	return result, err
}

// getGatewayEndpoint returns the external endpoint for the gateway
func (r *AgentGatewayReconciler) getGatewayEndpoint(gateway *runtimev1alpha1.AgentGateway) string {
	scheme := "http"
	if gateway.Spec.TLS.Enabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, gateway.Spec.Domain)
}

// isSecurityConfigured checks if IAP and TLS are properly configured
func (r *AgentGatewayReconciler) isSecurityConfigured(gateway *runtimev1alpha1.AgentGateway) bool {
	// Check TLS configuration
	tlsConfigured := !gateway.Spec.TLS.Enabled ||
		(gateway.Spec.TLS.Enabled && gateway.Spec.TLS.SecretName != "" && len(gateway.Spec.TLS.Hosts) > 0)

	// Check IAP configuration
	iapConfigured := !gateway.Spec.IAP.Enabled ||
		(gateway.Spec.IAP.Enabled && gateway.Spec.IAP.ClientID != "" &&
			(len(gateway.Spec.IAP.AllowedUsers) > 0 || len(gateway.Spec.IAP.AllowedDomains) > 0))

	return tlsConfigured && iapConfigured
}
