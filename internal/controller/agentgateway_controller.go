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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/agentgateway"
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
		Owns(&corev1.ConfigMap{}).
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
