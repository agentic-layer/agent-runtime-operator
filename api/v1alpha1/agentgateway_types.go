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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoutingStrategy defines how traffic should be routed to agents
type RoutingStrategy string

const (
	// PathBasedRouting routes traffic based on URL path prefixes
	PathBasedRouting RoutingStrategy = "path"
	// SubdomainBasedRouting routes traffic based on subdomains
	SubdomainBasedRouting RoutingStrategy = "subdomain"
)

// GatewayProvider defines the underlying gateway technology
type GatewayProvider string

const (
	// KrakenDProvider uses KrakenD as the gateway implementation
	KrakenDProvider GatewayProvider = "krakend"
)

// AgentReference defines a reference to an Agent resource that should be exposed
type AgentReference struct {
	// Name of the Agent resource
	Name string `json:"name"`

	// Namespace of the Agent resource (defaults to same namespace as AgentGateway)
	Namespace string `json:"namespace,omitempty"`

	// RoutePrefix defines the path prefix or subdomain for this agent
	// For path-based routing: "/weather" -> routes /weather/* to this agent
	// For subdomain-based routing: "weather" -> routes weather.domain.com to this agent
	RoutePrefix string `json:"routePrefix"`

	// TargetProtocol specifies which protocol of the agent to target
	// If empty, defaults to the first protocol defined in the Agent spec
	TargetProtocol string `json:"targetProtocol,omitempty"`

	// AllowedMethods defines the HTTP methods that should be routed to this agent
	// If empty, defaults to protocol-appropriate methods (e.g., GET for most, POST for OpenAI)
	// +kubebuilder:validation:items:Enum=GET;POST;PUT;DELETE;PATCH;HEAD;OPTIONS
	AllowedMethods []string `json:"allowedMethods,omitempty"`

	// Enabled allows selective enabling/disabling of agent exposure
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// IAPConfig defines Identity-Aware Proxy configuration
type IAPConfig struct {
	// Enabled determines if IAP should be configured
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// ClientID is the OAuth 2.0 client ID for IAP
	ClientID string `json:"clientId,omitempty"`

	// ClientSecret is the OAuth 2.0 client secret for IAP
	// Should reference a Kubernetes Secret
	ClientSecretRef *SecretReference `json:"clientSecretRef,omitempty"`

	// AllowedUsers defines the list of users allowed through IAP
	AllowedUsers []string `json:"allowedUsers,omitempty"`

	// AllowedDomains defines the list of email domains allowed through IAP
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

// SecretReference defines a reference to a Kubernetes Secret
type SecretReference struct {
	// Name of the secret
	Name string `json:"name"`

	// Key within the secret
	Key string `json:"key"`

	// Namespace of the secret (defaults to same namespace as AgentGateway)
	Namespace string `json:"namespace,omitempty"`
}

// TLSConfig defines TLS configuration for the gateway
type TLSConfig struct {
	// Enabled determines if TLS should be configured
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// SecretName is the name of the Kubernetes Secret containing TLS certificate and key
	SecretName string `json:"secretName,omitempty"`

	// Hosts defines the list of hostnames covered by the TLS certificate
	Hosts []string `json:"hosts,omitempty"`
}

// GatewayConfig defines provider-specific gateway configuration
type GatewayConfig struct {
	// Provider specifies the gateway technology to use
	// +kubebuilder:validation:Enum=krakend
	// +kubebuilder:default=krakend
	Provider GatewayProvider `json:"provider,omitempty"`

	// Image specifies the container image for the gateway
	Image string `json:"image,omitempty"`

	// Replicas is the number of gateway replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines compute resources for the gateway
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Config contains provider-specific configuration as key-value pairs
	Config map[string]string `json:"config,omitempty"`
}

// ResourceRequirements defines compute resource requirements
type ResourceRequirements struct {
	// Requests defines minimum required resources
	Requests map[string]string `json:"requests,omitempty"`

	// Limits defines maximum allowed resources
	Limits map[string]string `json:"limits,omitempty"`
}

// AgentGatewaySpec defines the desired state of AgentGateway
type AgentGatewaySpec struct {
	// Domain is the base domain for the gateway (e.g., "agents.example.com")
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`
	Domain string `json:"domain"`

	// RoutingStrategy defines how traffic should be routed to agents
	// +kubebuilder:validation:Enum=path;subdomain
	// +kubebuilder:default=path
	RoutingStrategy RoutingStrategy `json:"routingStrategy,omitempty"`

	// Agents defines the list of Agent resources to expose through this gateway
	Agents []AgentReference `json:"agents"`

	// Gateway defines the gateway provider and configuration
	Gateway GatewayConfig `json:"gateway,omitempty"`

	// IAP defines Identity-Aware Proxy configuration
	IAP IAPConfig `json:"iap,omitempty"`

	// TLS defines TLS configuration for the gateway
	TLS TLSConfig `json:"tls,omitempty"`

	// Annotations to be added to the gateway resources
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to be added to the gateway resources
	Labels map[string]string `json:"labels,omitempty"`
}

// GatewayConditionType defines the type of condition for the gateway
type GatewayConditionType string

const (
	// GatewayReady indicates the gateway is ready to serve traffic
	GatewayReady GatewayConditionType = "Ready"
	// GatewayConfigured indicates the gateway configuration has been applied
	GatewayConfigured GatewayConditionType = "Configured"
	// GatewaySecured indicates IAP and TLS have been properly configured
	GatewaySecured GatewayConditionType = "Secured"
)

// AgentGatewayStatus defines the observed state of AgentGateway
type AgentGatewayStatus struct {
	// Conditions represent the latest available observations of the gateway's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// ExposedAgents lists the agents currently exposed through this gateway
	ExposedAgents []string `json:"exposedAgents,omitempty"`

	// GatewayEndpoint is the external endpoint where the gateway can be reached
	GatewayEndpoint string `json:"gatewayEndpoint,omitempty"`

	// ConfigHash represents the hash of the current gateway configuration
	ConfigHash string `json:"configHash,omitempty"`

	// LastUpdated is the timestamp when the gateway was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Domain",type="string",JSONPath=".spec.domain"
// +kubebuilder:printcolumn:name="Strategy",type="string",JSONPath=".spec.routingStrategy"
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.gateway.provider"
// +kubebuilder:printcolumn:name="Agents",type="integer",JSONPath=".status.exposedAgents[*]",description="Number of exposed agents"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AgentGateway is the Schema for the agentgateways API
type AgentGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentGatewaySpec   `json:"spec,omitempty"`
	Status AgentGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentGatewayList contains a list of AgentGateway
type AgentGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentGateway{}, &AgentGatewayList{})
}
