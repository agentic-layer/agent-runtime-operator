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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// AgentGatewayReconciler reconciles an AgentGateway object
type AgentGatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// KrakendBackend represents a backend configuration in KrakenD
type KrakendBackend struct {
	Host       []string `json:"host"`
	URLPattern string   `json:"url_pattern"`
}

// KrakendEndpoint represents an endpoint configuration in KrakenD
type KrakendEndpoint struct {
	Endpoint       string           `json:"endpoint"`
	OutputEncoding string           `json:"output_encoding"`
	Method         string           `json:"method"`
	Backend        []KrakendBackend `json:"backend"`
}

// defaultKrakendConfig is the base KrakenD configuration template
const defaultKrakendConfig = `{
    "$schema": "https://www.krakend.io/schema/v2.10/krakend.json",
    "version": 3,
    "port": 8080,
    "extra_config": {
        "telemetry/logging": {
            "level": "DEBUG",
            "syslog": true,
            "stdout": true
        },
        "router": {
            "disable_access_log": false,
            "hide_version_header": false
        }
    },
    "timeout": "60000ms",
    "cache_ttl": "300s",
    "output_encoding": "json",
    "name": "agent-gateway-krakend",
    "endpoints": []
}`

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

	// Check if deployment already exists
	deployment := &appsv1.Deployment{}
	deploymentName := agentGateway.Name
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		// Deployment does not exist, create it
		log.Info("Creating new Deployment", "name", deploymentName, "namespace", agentGateway.Namespace)

		deployment, err = r.createDeploymentForKrakendAgentGateway(ctx, &agentGateway, deploymentName)
		if err != nil {
			log.Error(err, "Failed to create deployment for Agent")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create new Deployment", "name", deploymentName)
			return ctrl.Result{}, err
		}

		log.Info("Successfully created Deployment", "name", deploymentName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	service := &corev1.Service{}
	serviceName := agentGateway.Name
	err = r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agentGateway.Namespace}, service)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new Service", "name", serviceName, "namespace", agentGateway.Namespace)

		service, err = r.createServiceForAgentGateway(&agentGateway, serviceName)
		if err != nil {
			log.Error(err, "Failed to create service for Agent Gateway")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, service); err != nil {
			log.Error(err, "Failed to create new Service", "name", serviceName)
			return ctrl.Result{}, err
		}

		log.Info("Successfully created Service", "name", serviceName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	//// Step 1: Validate gateway configuration early
	//if err := r.validateGatewayConfig(&agentGateway); err != nil {
	//	log.Error(err, "Invalid AgentGateway configuration")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayConfigured, metav1.ConditionFalse,
	//		"ConfigurationInvalid", fmt.Sprintf("Gateway configuration validation failed: %v", err))
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"ConfigurationInvalid", "Gateway not ready due to invalid configuration")
	//	// Configuration errors require user intervention - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 2: Discover and validate referenced Agent resources
	//discoveredAgents, err := r.discoverAgents(ctx, &agentGateway)
	//if err != nil {
	//	log.Error(err, "Failed to discover agents")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayConfigured, metav1.ConditionFalse,
	//		"AgentDiscoveryFailed", fmt.Sprintf("Failed to discover agents: %v", err))
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"AgentDiscoveryFailed", "Gateway not ready due to agent discovery failure")
	//	// Agent discovery failures are dependency errors - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 3: Generate gateway configuration using new generator pattern
	//configData, configHash, err := r.generateGatewayConfigNew(ctx, &agentGateway, discoveredAgents)
	//if err != nil {
	//	log.Error(err, "Failed to generate gateway configuration")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayConfigured, metav1.ConditionFalse,
	//		"ConfigGenerationFailed", fmt.Sprintf("Failed to generate config: %v", err))
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"ConfigGenerationFailed", "Gateway not ready due to config generation failure")
	//	// Configuration generation failures - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 4: Create/update ConfigMap with gateway configuration
	//if err := r.reconcileConfigMap(ctx, &agentGateway, configData, configHash); err != nil {
	//	log.Error(err, "Failed to reconcile ConfigMap")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayConfigured, metav1.ConditionFalse,
	//		"ConfigMapFailed", fmt.Sprintf("Failed to create/update ConfigMap: %v", err))
	//	// ConfigMap operations are typically transient errors - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 5: Create/update Deployment
	//if err := r.reconcileDeployment(ctx, &agentGateway, configHash); err != nil {
	//	log.Error(err, "Failed to reconcile Deployment")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"DeploymentFailed", fmt.Sprintf("Failed to create/update Deployment: %v", err))
	//	// Deployment operations are typically transient errors - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 6: Create/update Service
	//if err := r.reconcileService(ctx, &agentGateway); err != nil {
	//	log.Error(err, "Failed to reconcile Service")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"ServiceFailed", fmt.Sprintf("Failed to create/update Service: %v", err))
	//	// Service operations are typically transient errors - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Step 7: Create/update Ingress
	//if err := r.reconcileIngress(ctx, &agentGateway); err != nil {
	//	log.Error(err, "Failed to reconcile Ingress")
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionFalse,
	//		"IngressFailed", fmt.Sprintf("Failed to create/update Ingress: %v", err))
	//	// Ingress operations are typically transient errors - use intelligent error handling
	//	result, statusErr := r.ErrorClassifier.HandleReconcileError(err)
	//	return r.updateStatus(ctx, &agentGateway, result, statusErr)
	//}
	//
	//// Update successful status
	//exposedAgentNames := make([]string, 0, len(discoveredAgents))
	//for _, agent := range discoveredAgents {
	//	exposedAgentNames = append(exposedAgentNames, agent.Name)
	//}
	//sort.Strings(exposedAgentNames)
	//
	//agentGateway.Status.ExposedAgents = exposedAgentNames
	//agentGateway.Status.ConfigHash = configHash
	//agentGateway.Status.GatewayEndpoint = r.getGatewayEndpoint(&agentGateway)
	//now := metav1.Now()
	//agentGateway.Status.LastUpdated = &now
	//
	//r.updateCondition(&agentGateway, runtimev1alpha1.GatewayConfigured, metav1.ConditionTrue,
	//	"ConfigurationApplied", "Gateway configuration successfully applied")
	//
	//// Check if IAP/TLS are properly configured
	//if r.isSecurityConfigured(&agentGateway) {
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewaySecured, metav1.ConditionTrue,
	//		"SecurityConfigured", "IAP and TLS properly configured")
	//} else {
	//	r.updateCondition(&agentGateway, runtimev1alpha1.GatewaySecured, metav1.ConditionFalse,
	//		"SecurityNotConfigured", "IAP or TLS not fully configured")
	//}
	//
	//r.updateCondition(&agentGateway, runtimev1alpha1.GatewayReady, metav1.ConditionTrue,
	//	"GatewayReady", "Gateway is ready and serving traffic")
	//
	//log.Info("Successfully reconciled AgentGateway", "name", agentGateway.Name,
	//	"exposedAgents", len(exposedAgentNames), "configHash", configHash[:8])

	return ctrl.Result{}, nil
}

// createDeploymentForAgent creates a deployment for the given Agent
func (r *AgentGatewayReconciler) createDeploymentForKrakendAgentGateway(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, deploymentName string) (*appsv1.Deployment, error) {
	replicas := agentGateway.Spec.Replicas
	if replicas == nil {
		replicas = new(int32)
		*replicas = 1 // Default to 1 replica if not specified
	}

	labels := map[string]string{
		"app":      agentGateway.Name,
		"provider": string(agentGateway.Spec.Provider),
	}

	// Selector labels should be stable (immutable in Kubernetes)
	selectorLabels := map[string]string{
		"app": agentGateway.Name,
	}

	// Create container port
	containerPort := corev1.ContainerPort{
		Name:          "http",
		ContainerPort: 8080,
		Protocol:      corev1.ProtocolTCP,
	}

	configMap, err := r.createConfigMapForKrakendGateway(ctx, agentGateway)
	if err != nil {
		return nil, err
	}

	// Create the krakend config volume
	configVolume := corev1.Volume{
		Name: "krakend-config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: agentGateway.Namespace,
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
							Name:  "agent-gateway",
							Image: "eu.gcr.io/agentic-layer/agent-gateway-krakend:main",
							Ports: []corev1.ContainerPort{containerPort},
							Resources: corev1.ResourceRequirements{ //TODO Make configurable
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("32Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("64Mi"),
									corev1.ResourceCPU:    resource.MustParse("500m"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configVolume.Name,
									MountPath: "/etc/krakend",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						configVolume,
					},
				},
			},
		},
	}

	// Set Agent as the owner of the Deployment
	if err := ctrl.SetControllerReference(agentGateway, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return deployment, nil
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
		TargetPort: intstr.FromInt32(8080), // Target the container port 8080
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

// createConfigMapForKrakendGateway creates a ConfigMap with KrakenD configuration
func (r *AgentGatewayReconciler) createConfigMapForKrakendGateway(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway) (*corev1.ConfigMap, error) {

	// Parse the base KrakenD configuration from embedded template
	var krakendConfigMap map[string]interface{}
	if err := json.Unmarshal([]byte(defaultKrakendConfig), &krakendConfigMap); err != nil {
		return nil, fmt.Errorf("failed to parse KrakenD config: %w", err)
	}

	// Get all exposed agents
	exposedAgents, err := r.getExposedAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get exposed agents: %w", err)
	}

	// Generate endpoints for all exposed agents
	endpoints := make([]KrakendEndpoint, 0, len(exposedAgents))
	for _, agent := range exposedAgents {
		endpoint, err := r.generateEndpointForAgent(ctx, agent)
		if err != nil {
			return nil, fmt.Errorf("failed to generate endpoint for agent %s: %w", agent.Name, err)
		}
		endpoints = append(endpoints, endpoint)
	}

	// Add the generated endpoints to the KrakenD config
	krakendConfigMap["endpoints"] = endpoints

	// Marshal the updated configuration back to JSON
	updatedConfigBytes, err := json.MarshalIndent(krakendConfigMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated KrakenD config: %w", err)
	}
	krakendConfig := string(updatedConfigBytes)

	labels := map[string]string{
		"app":      agentGateway.Name,
		"provider": string(agentGateway.Spec.Provider),
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "krakend-config",
			Namespace: agentGateway.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"krakend.json": krakendConfig,
		},
	}

	// Set AgentGateway as the owner of the ConfigMap
	if err := ctrl.SetControllerReference(agentGateway, configMap, r.Scheme); err != nil {
		return nil, err
	}

	return configMap, nil
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

// getAgentServiceURL finds the service owned by the agent and generates the Kubernetes service URL
func (r *AgentGatewayReconciler) getAgentServiceURL(ctx context.Context, agent *runtimev1alpha1.Agent) (string, error) {
	var serviceList corev1.ServiceList
	if err := r.List(ctx, &serviceList, client.InNamespace(agent.Namespace)); err != nil {
		return "", fmt.Errorf("failed to list services in namespace %s: %w", agent.Namespace, err)
	}

	var foundService *corev1.Service
	for i := range serviceList.Items {
		service := &serviceList.Items[i]
		for _, ownerRef := range service.OwnerReferences {
			if ownerRef.Kind == "Agent" && ownerRef.Name == agent.Name && ownerRef.UID == agent.UID {
				foundService = service
				break
			}
		}
		if foundService != nil {
			break
		}
	}

	if foundService == nil {
		return "", fmt.Errorf("no service found owned by agent %s in namespace %s", agent.Name, agent.Namespace)
	}

	if len(foundService.Spec.Ports) == 0 {
		return "", fmt.Errorf("service %s owned by agent %s has no ports defined", foundService.Name, agent.Name)
	}

	port := foundService.Spec.Ports[0].Port
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", foundService.Name, foundService.Namespace, port), nil
}

// generateEndpointForAgent creates the endpoint JSON configuration for a single agent
func (r *AgentGatewayReconciler) generateEndpointForAgent(ctx context.Context, agent *runtimev1alpha1.Agent) (KrakendEndpoint, error) {
	serviceURL, err := r.getAgentServiceURL(ctx, agent)
	if err != nil {
		return KrakendEndpoint{}, fmt.Errorf("failed to get service URL for agent %s: %w", agent.Name, err)
	}

	return KrakendEndpoint{
		Endpoint:       fmt.Sprintf("/%s/{anyPath}/", agent.Name),
		OutputEncoding: "no-op",
		Method:         "POST",
		Backend: []KrakendBackend{
			{
				Host:       []string{serviceURL},
				URLPattern: "/{anyPath}/",
			},
		},
	}, nil
}

// // discoverAgents finds and validates all Agent resources referenced by the AgentGateway
//
//	func (r *AgentGatewayReconciler) discoverAgents(ctx context.Context, gateway *runtimev1alpha1.AgentGateway) ([]*runtimev1alpha1.Agent, error) {
//		var discoveredAgents []*runtimev1alpha1.Agent
//
//		for _, agentRef := range gateway.Spec.Agents {
//			if !agentRef.Enabled {
//				continue
//			}
//
//			// Determine namespace - default to gateway's namespace if not specified
//			namespace := agentRef.Namespace
//			if namespace == "" {
//				namespace = gateway.Namespace
//			}
//
//			// Fetch the Agent resource
//			var agent runtimev1alpha1.Agent
//			key := types.NamespacedName{Name: agentRef.Name, Namespace: namespace}
//			if err := r.Get(ctx, key, &agent); err != nil {
//				if errors.IsNotFound(err) {
//					return nil, fmt.Errorf("agent %s not found in namespace %s", agentRef.Name, namespace)
//				}
//				return nil, fmt.Errorf("failed to get agent %s: %w", agentRef.Name, err)
//			}
//
//			// Validate that the agent has the required protocol
//			if agentRef.TargetProtocol != "" {
//				found := false
//				for _, protocol := range agent.Spec.Protocols {
//					if protocol.Type == agentRef.TargetProtocol {
//						found = true
//						break
//					}
//				}
//				if !found {
//					return nil, fmt.Errorf("agent %s does not support protocol %s", agentRef.Name, agentRef.TargetProtocol)
//				}
//			} else if len(agent.Spec.Protocols) == 0 {
//				return nil, fmt.Errorf("agent %s has no protocols defined", agentRef.Name)
//			}
//
//			discoveredAgents = append(discoveredAgents, &agent)
//		}
//
//		return discoveredAgents, nil
//	}
//
// // generateGatewayConfig creates the configuration for the gateway based on the provider
//
//	func (r *AgentGatewayReconciler) generateGatewayConfig(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, agents []*runtimev1alpha1.Agent) (string, string, error) {
//		provider := gateway.Spec.Gateway.Provider
//		if provider == "" {
//			provider = runtimev1alpha1.KrakenDProvider
//		}
//
//		switch provider {
//		case runtimev1alpha1.KrakenDProvider:
//			return r.generateKrakenDConfig(ctx, gateway, agents)
//		default:
//			return "", "", fmt.Errorf("unsupported gateway provider: %s", provider)
//		}
//	}
//
// // generateKrakenDConfig generates KrakenD JSON configuration
//
//	func (r *AgentGatewayReconciler) generateKrakenDConfig(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, agents []*runtimev1alpha1.Agent) (string, string, error) {
//		config := map[string]interface{}{
//			"version":   3,
//			"name":      fmt.Sprintf("agent-gateway-%s", gateway.Name),
//			"port":      8080,
//			"host":      []string{"https://" + gateway.Spec.Domain},
//			"timeout":   "5s",
//			"cache_ttl": "300s",
//			"endpoints": []map[string]interface{}{},
//		}
//
//		// Add custom config values
//		if gateway.Spec.Gateway.Config != nil {
//			for key, value := range gateway.Spec.Gateway.Config {
//				switch key {
//				case "timeout", "cache_ttl":
//					config[key] = value
//				}
//			}
//		}
//
//		endpoints := []map[string]interface{}{}
//
//		// Generate endpoints for each agent
//		for i, agent := range agents {
//			agentRef := gateway.Spec.Agents[i]
//			if !agentRef.Enabled {
//				continue
//			}
//
//			// Get the target protocol and port
//			protocol := r.getTargetProtocol(agent, agentRef.TargetProtocol)
//			if protocol == nil {
//				continue
//			}
//
//			port := protocol.Port
//			if port == 0 {
//				// Set default ports based on protocol type
//				switch protocol.Type {
//				case "A2A":
//					port = 8080
//				case "OpenAI":
//					port = 8000
//				default:
//					port = 8080
//				}
//			}
//
//			// Generate endpoint path based on routing strategy
//			var endpointPath string
//			var upstreamURL string
//
//			namespace := agentRef.Namespace
//			if namespace == "" {
//				namespace = gateway.Namespace
//			}
//
//			serviceName := fmt.Sprintf("%s.%s.svc.cluster.local", agent.Name, namespace)
//			upstreamURL = fmt.Sprintf("http://%s:%d", serviceName, port)
//
//			if protocol.Path != "" {
//				upstreamURL += protocol.Path
//			}
//
//			switch gateway.Spec.RoutingStrategy {
//			case runtimev1alpha1.PathBasedRouting:
//				endpointPath = agentRef.RoutePrefix + "/*"
//			case runtimev1alpha1.SubdomainBasedRouting:
//				// For subdomain routing, we'll use path matching in KrakenD
//				// The Ingress will handle subdomain routing to the gateway
//				endpointPath = "/*"
//			default:
//				endpointPath = agentRef.RoutePrefix + "/*"
//			}
//
//			endpoint := map[string]interface{}{
//				"endpoint": endpointPath,
//				"method":   "GET",
//				"backend": []map[string]interface{}{
//					{
//						"url_pattern": "/",
//						"host":        []string{upstreamURL},
//						"method":      "GET",
//					},
//				},
//			}
//
//			// Support other HTTP methods
//			if protocol.Type == "OpenAI" {
//				endpoint["method"] = "POST"
//				endpoint["backend"].([]map[string]interface{})[0]["method"] = "POST"
//			}
//
//			endpoints = append(endpoints, endpoint)
//		}
//
//		config["endpoints"] = endpoints
//
//		// Generate JSON
//		configJSON, err := json.MarshalIndent(config, "", "  ")
//		if err != nil {
//			return "", "", fmt.Errorf("failed to marshal KrakenD config: %w", err)
//		}
//
//		// Generate config hash
//		hash := sha256.Sum256(configJSON)
//		configHash := fmt.Sprintf("%x", hash)[:16]
//
//		return string(configJSON), configHash, nil
//	}
//
// // getTargetProtocol returns the target protocol for an agent
//
//	func (r *AgentGatewayReconciler) getTargetProtocol(agent *runtimev1alpha1.Agent, targetProtocol string) *runtimev1alpha1.AgentProtocol {
//		if targetProtocol != "" {
//			for _, protocol := range agent.Spec.Protocols {
//				if protocol.Type == targetProtocol {
//					return &protocol
//				}
//			}
//			return nil
//		}
//
//		// Return first protocol if no specific target
//		if len(agent.Spec.Protocols) > 0 {
//			return &agent.Spec.Protocols[0]
//		}
//
//		return nil
//	}
//
// // reconcileConfigMap creates or updates the ConfigMap containing gateway configuration
//
//	func (r *AgentGatewayReconciler) reconcileConfigMap(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, configData, configHash string) error {
//		log := logf.FromContext(ctx)
//
//		configMap := &corev1.ConfigMap{
//			ObjectMeta: metav1.ObjectMeta{
//				Name:      fmt.Sprintf("%s-config", gateway.Name),
//				Namespace: gateway.Namespace,
//				Labels: map[string]string{
//					"app.kubernetes.io/name":               "agentgateway",
//					"app.kubernetes.io/instance":           gateway.Name,
//					"app.kubernetes.io/managed-by":         "agent-runtime-operator",
//					"gateway.agentic-layer.ai/config-hash": configHash,
//				},
//			},
//			Data: map[string]string{
//				"krakend.json": configData,
//			},
//		}
//
//		// Add custom labels if specified
//		if gateway.Spec.Labels != nil {
//			for k, v := range gateway.Spec.Labels {
//				configMap.Labels[k] = v
//			}
//		}
//
//		// Add custom annotations if specified
//		if gateway.Spec.Annotations != nil {
//			if configMap.Annotations == nil {
//				configMap.Annotations = make(map[string]string)
//			}
//			for k, v := range gateway.Spec.Annotations {
//				configMap.Annotations[k] = v
//			}
//		}
//
//		// Set owner reference
//		if err := controllerutil.SetControllerReference(gateway, configMap, r.Scheme); err != nil {
//			return fmt.Errorf("failed to set owner reference: %w", err)
//		}
//
//		// Check if ConfigMap exists
//		existing := &corev1.ConfigMap{}
//		err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existing)
//
//		if err != nil && errors.IsNotFound(err) {
//			log.Info("Creating ConfigMap", "name", configMap.Name)
//			return r.Create(ctx, configMap)
//		} else if err != nil {
//			return fmt.Errorf("failed to get ConfigMap: %w", err)
//		}
//
//		// Update if needed
//		if existing.Data["krakend.json"] != configData {
//			log.Info("Updating ConfigMap", "name", configMap.Name)
//			existing.Data = configMap.Data
//			existing.Labels = configMap.Labels
//			existing.Annotations = configMap.Annotations
//			return r.Update(ctx, existing)
//		}
//
//		return nil
//	}
//
// SetupWithManager sets up the controller with the Manager.
func (r *AgentGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&runtimev1alpha1.AgentGateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("agentgateway").
		Complete(r)
}

//
//// validateGatewayConfig validates the gateway configuration before processing
//func (r *AgentGatewayReconciler) validateGatewayConfig(gateway *runtimev1alpha1.AgentGateway) error {
//	provider := gateway.Spec.Gateway.Provider
//	if provider == "" {
//		provider = runtimev1alpha1.KrakenDProvider
//	}
//
//	generator, err := NewGatewayGenerator(provider)
//	if err != nil {
//		return fmt.Errorf("failed to create gateway generator: %w", err)
//	}
//
//	return generator.Validate(gateway)
//}
//
//// generateGatewayConfigNew creates the configuration using the new generator pattern
//func (r *AgentGatewayReconciler) generateGatewayConfigNew(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, agents []*runtimev1alpha1.Agent) (string, string, error) {
//	provider := gateway.Spec.Gateway.Provider
//	if provider == "" {
//		provider = runtimev1alpha1.KrakenDProvider
//	}
//
//	generator, err := NewGatewayGenerator(provider)
//	if err != nil {
//		return "", "", fmt.Errorf("failed to create gateway generator: %w", err)
//	}
//
//	return generator.Generate(ctx, gateway, agents)
//}
