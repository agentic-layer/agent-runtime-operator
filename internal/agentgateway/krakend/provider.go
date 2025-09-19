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

package krakend

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

// Provider implements the AgentGatewayProvider interface for KrakenD
type Provider struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewProvider creates a new KrakenD provider instance
func NewProvider(client client.Client, scheme *runtime.Scheme) *Provider {
	return &Provider{
		client: client,
		scheme: scheme,
	}
}

// CreateAgentGatewayResources creates all necessary resources for the KrakenD gateway
func (p *Provider) CreateAgentGatewayResources(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, exposedAgents []*runtimev1alpha1.Agent) error {
	log := logf.FromContext(ctx)

	// Create ConfigMap
	configMapName := agentGateway.Name + "-krakend-config"
	if err := p.ensureConfigMap(ctx, agentGateway, configMapName, exposedAgents); err != nil {
		log.Error(err, "Failed to ensure ConfigMap for KrakenD")
		return err
	}

	// Create Deployment
	deploymentName := agentGateway.Name
	if err := p.ensureDeployment(ctx, agentGateway, deploymentName, configMapName); err != nil {
		log.Error(err, "Failed to ensure Deployment for KrakenD")
		return err
	}

	// Create Service
	serviceName := agentGateway.Name
	if err := p.ensureService(ctx, agentGateway, serviceName); err != nil {
		log.Error(err, "Failed to ensure Service for KrakenD")
		return err
	}

	return nil
}

// ensureConfigMap creates or updates the ConfigMap with KrakenD configuration
func (p *Provider) ensureConfigMap(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, configMapName string, exposedAgents []*runtimev1alpha1.Agent) error {
	log := logf.FromContext(ctx)

	configMap := &corev1.ConfigMap{}
	err := p.client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)

	if err != nil && errors.IsNotFound(err) {
		configMap, err := p.createConfigMapForKrakend(ctx, agentGateway, configMapName, exposedAgents)
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}

		if err := p.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create new ConfigMap %s: %w", configMapName, err)
		}

		log.Info("Successfully created ConfigMap", "name", configMapName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	return nil
}

// ensureDeployment creates or updates the KrakenD deployment
func (p *Provider) ensureDeployment(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, deploymentName string, configMapName string) error {
	log := logf.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	err := p.client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		deployment, err := p.createDeploymentForKrakend(agentGateway, deploymentName, configMapName)
		if err != nil {
			return fmt.Errorf("failed to create Deployment: %w", err)
		}

		if err := p.client.Create(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create new Deployment %s: %w", deploymentName, err)
		}

		log.Info("Successfully created Deployment", "name", deploymentName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	return nil
}

// ensureService creates or updates the KrakenD service
func (p *Provider) ensureService(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, serviceName string) error {
	log := logf.FromContext(ctx)

	service := &corev1.Service{}
	err := p.client.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: agentGateway.Namespace}, service)

	if err != nil && errors.IsNotFound(err) {
		service, err := p.createServiceForKrakend(agentGateway, serviceName)
		if err != nil {
			return fmt.Errorf("failed to create Service: %w", err)
		}

		if err := p.client.Create(ctx, service); err != nil {
			return fmt.Errorf("failed to create new Service %s: %w", serviceName, err)
		}

		log.Info("Successfully created Service", "name", serviceName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get Service: %w", err)
	}

	return nil
}

// createConfigMapForKrakend creates a ConfigMap with KrakenD configuration
func (p *Provider) createConfigMapForKrakend(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, configMapName string, exposedAgents []*runtimev1alpha1.Agent) (*corev1.ConfigMap, error) {
	// Parse the base KrakenD configuration from embedded template
	var krakendConfigMap map[string]interface{}
	if err := json.Unmarshal([]byte(defaultKrakendConfig), &krakendConfigMap); err != nil {
		return nil, fmt.Errorf("failed to parse KrakenD config: %w", err)
	}

	// Generate endpoints for all exposed agents
	endpoints := make([]KrakendEndpoint, 0, len(exposedAgents))
	for _, agent := range exposedAgents {
		endpoint, err := p.generateEndpointForAgent(ctx, agent)
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
			Name:      configMapName,
			Namespace: agentGateway.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"krakend.json": krakendConfig,
		},
	}

	// Set AgentGateway as the owner of the ConfigMap
	if err := ctrl.SetControllerReference(agentGateway, configMap, p.scheme); err != nil {
		return nil, err
	}

	return configMap, nil
}

// createDeploymentForKrakend creates a deployment for KrakenD
func (p *Provider) createDeploymentForKrakend(agentGateway *runtimev1alpha1.AgentGateway, deploymentName string, configMapName string) (*appsv1.Deployment, error) {
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

	// Create the krakend config volume
	configVolume := corev1.Volume{
		Name: "krakend-config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
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
							Resources: corev1.ResourceRequirements{
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

	// Set AgentGateway as the owner of the Deployment
	if err := ctrl.SetControllerReference(agentGateway, deployment, p.scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

// createServiceForKrakend creates a service for KrakenD
func (p *Provider) createServiceForKrakend(agentGateway *runtimev1alpha1.AgentGateway, serviceName string) (*corev1.Service, error) {
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
	if err := ctrl.SetControllerReference(agentGateway, service, p.scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// generateEndpointForAgent creates the endpoint JSON configuration for a single agent
func (p *Provider) generateEndpointForAgent(ctx context.Context, agent *runtimev1alpha1.Agent) (KrakendEndpoint, error) {
	serviceURL, err := p.getAgentServiceURL(ctx, agent)
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

// getAgentServiceURL finds the service owned by the agent and generates the Kubernetes service URL
func (p *Provider) getAgentServiceURL(ctx context.Context, agent *runtimev1alpha1.Agent) (string, error) {
	var serviceList corev1.ServiceList
	if err := p.client.List(ctx, &serviceList, client.InNamespace(agent.Namespace)); err != nil {
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
