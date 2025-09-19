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
	"bytes"
	"context"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/constants"
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

// KrakendConfigData holds the data for template execution
type KrakendConfigData struct {
	Port      int32
	Timeout   string
	CacheTTL  string
	Endpoints []KrakendEndpoint
}

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

	return nil
}

// ensureConfigMap creates or updates the ConfigMap with KrakenD configuration
func (p *Provider) ensureConfigMap(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, configMapName string, exposedAgents []*runtimev1alpha1.Agent) error {
	log := logf.FromContext(ctx)

	// First, generate the desired ConfigMap configuration
	desiredConfigMap, err := p.createConfigMapForKrakend(ctx, agentGateway, configMapName, exposedAgents)
	if err != nil {
		return fmt.Errorf("failed to generate desired ConfigMap: %w", err)
	}

	// Try to get existing ConfigMap
	existingConfigMap := &corev1.ConfigMap{}
	err = p.client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, existingConfigMap)

	if err != nil && errors.IsNotFound(err) {
		// ConfigMap doesn't exist, create it
		if err := p.client.Create(ctx, desiredConfigMap); err != nil {
			return fmt.Errorf("failed to create new ConfigMap %s: %w", configMapName, err)
		}
		log.Info("Successfully created ConfigMap", "name", configMapName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	} else {
		// ConfigMap exists, check if update is needed
		if p.configMapNeedsUpdate(existingConfigMap, desiredConfigMap) {
			// Update the existing ConfigMap's data and labels
			existingConfigMap.Data = desiredConfigMap.Data
			existingConfigMap.Labels = desiredConfigMap.Labels

			if err := p.client.Update(ctx, existingConfigMap); err != nil {
				return fmt.Errorf("failed to update ConfigMap %s: %w", configMapName, err)
			}
			log.Info("Successfully updated ConfigMap", "name", configMapName, "namespace", agentGateway.Namespace)
		} else {
			log.V(1).Info("ConfigMap is up to date, no update needed", "name", configMapName, "namespace", agentGateway.Namespace)
		}
	}

	return nil
}

// ensureDeployment creates or updates the KrakenD deployment
func (p *Provider) ensureDeployment(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, deploymentName string, configMapName string) error {
	log := logf.FromContext(ctx)

	// First, generate the desired Deployment configuration
	desiredDeployment, err := p.createDeploymentForKrakend(agentGateway, deploymentName, configMapName)
	if err != nil {
		return fmt.Errorf("failed to generate desired Deployment: %w", err)
	}

	// Try to get existing Deployment
	existingDeployment := &appsv1.Deployment{}
	err = p.client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, existingDeployment)

	if err != nil && errors.IsNotFound(err) {
		// Deployment doesn't exist, create it
		if err := p.client.Create(ctx, desiredDeployment); err != nil {
			return fmt.Errorf("failed to create new Deployment %s: %w", deploymentName, err)
		}
		log.Info("Successfully created Deployment", "name", deploymentName, "namespace", agentGateway.Namespace)
	} else if err != nil {
		return fmt.Errorf("failed to get Deployment: %w", err)
	} else {
		// Deployment exists, check if update is needed
		if p.deploymentNeedsUpdate(existingDeployment, desiredDeployment) {
			// Update the existing Deployment's spec and labels
			existingDeployment.Labels = desiredDeployment.Labels
			existingDeployment.Spec.Replicas = desiredDeployment.Spec.Replicas
			existingDeployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
			existingDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes

			if err := p.client.Update(ctx, existingDeployment); err != nil {
				return fmt.Errorf("failed to update Deployment %s: %w", deploymentName, err)
			}
			log.Info("Successfully updated Deployment", "name", deploymentName, "namespace", agentGateway.Namespace)
		} else {
			log.V(1).Info("Deployment is up to date, no update needed", "name", deploymentName, "namespace", agentGateway.Namespace)
		}
	}

	return nil
}

// createConfigMapForKrakend creates a ConfigMap with KrakenD configuration
func (p *Provider) createConfigMapForKrakend(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, configMapName string, exposedAgents []*runtimev1alpha1.Agent) (*corev1.ConfigMap, error) {
	// Generate endpoints for all exposed agents
	endpoints := make([]KrakendEndpoint, 0, len(exposedAgents))
	for _, agent := range exposedAgents {
		endpoint, err := p.generateEndpointForAgent(ctx, agent)
		if err != nil {
			return nil, fmt.Errorf("failed to generate endpoint for agent %s: %w", agent.Name, err)
		}
		endpoints = append(endpoints, endpoint)
	}

	// Prepare template data with default values
	timeout := "60000ms"
	if agentGateway.Spec.Timeout != nil {
		timeout = *agentGateway.Spec.Timeout
	}

	cacheTTL := "300s"
	if agentGateway.Spec.CacheTTL != nil {
		cacheTTL = *agentGateway.Spec.CacheTTL
	}

	templateData := KrakendConfigData{
		Port:      constants.DefaultGatewayPort,
		Timeout:   timeout,
		CacheTTL:  cacheTTL,
		Endpoints: endpoints,
	}

	// Parse and execute the template
	tmpl, err := template.New("krakend").Parse(krakendConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse KrakenD template: %w", err)
	}

	var configBuffer bytes.Buffer
	if err := tmpl.Execute(&configBuffer, templateData); err != nil {
		return nil, fmt.Errorf("failed to execute KrakenD template: %w", err)
	}
	krakendConfig := configBuffer.String()

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
		ContainerPort: constants.DefaultGatewayPort,
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

// configMapNeedsUpdate compares existing and desired ConfigMaps to determine if an update is needed
func (p *Provider) configMapNeedsUpdate(existing, desired *corev1.ConfigMap) bool {
	// Compare data content
	if len(existing.Data) != len(desired.Data) {
		return true
	}

	for key, desiredValue := range desired.Data {
		if existingValue, exists := existing.Data[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	// Compare labels (optional but good practice for consistency)
	if len(existing.Labels) != len(desired.Labels) {
		return true
	}

	for key, desiredValue := range desired.Labels {
		if existingValue, exists := existing.Labels[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	return false
}

// deploymentNeedsUpdate compares existing and desired Deployments to determine if an update is needed
func (p *Provider) deploymentNeedsUpdate(existing, desired *appsv1.Deployment) bool {
	// Compare replica count
	existingReplicas := int32(1) // Default replica count
	if existing.Spec.Replicas != nil {
		existingReplicas = *existing.Spec.Replicas
	}

	desiredReplicas := int32(1) // Default replica count
	if desired.Spec.Replicas != nil {
		desiredReplicas = *desired.Spec.Replicas
	}

	if existingReplicas != desiredReplicas {
		return true
	}

	// Compare labels
	if len(existing.Labels) != len(desired.Labels) {
		return true
	}

	for key, desiredValue := range desired.Labels {
		if existingValue, exists := existing.Labels[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	// Compare template labels (pod labels)
	if len(existing.Spec.Template.Labels) != len(desired.Spec.Template.Labels) {
		return true
	}

	for key, desiredValue := range desired.Spec.Template.Labels {
		if existingValue, exists := existing.Spec.Template.Labels[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	// Compare ConfigMap volume reference (in case the ConfigMap name changes)
	existingConfigMapName := p.getConfigMapNameFromVolumes(existing.Spec.Template.Spec.Volumes)
	desiredConfigMapName := p.getConfigMapNameFromVolumes(desired.Spec.Template.Spec.Volumes)

	if existingConfigMapName != desiredConfigMapName {
		return true
	}

	return false
}

// getConfigMapNameFromVolumes extracts the ConfigMap name from the volumes
func (p *Provider) getConfigMapNameFromVolumes(volumes []corev1.Volume) string {
	for _, volume := range volumes {
		if volume.ConfigMap != nil {
			return volume.ConfigMap.Name
		}
	}
	return ""
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
