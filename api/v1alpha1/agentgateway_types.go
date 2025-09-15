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

// GatewayProvider defines the underlying gateway technology
type GatewayProvider string

const (
	// KrakenDProvider uses KrakenD as the gateway implementation
	KrakenDProvider GatewayProvider = "krakend"
)

// AgentGatewaySpec defines the desired state of AgentGateway
type AgentGatewaySpec struct {
	// Provider specifies the gateway technology to use
	// +kubebuilder:validation:Enum=krakend
	// +kubebuilder:default=krakend
	Provider GatewayProvider `json:"provider,omitempty"`

	// Replicas is the number of gateway replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	Replicas *int32 `json:"replicas,omitempty"`

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

	// GatewayEndpoint is the external endpoint where the gateway can be reached
	GatewayEndpoint string `json:"gatewayEndpoint,omitempty"`

	// ConfigHash represents the hash of the current gateway configuration
	ConfigHash string `json:"configHash,omitempty"`

	// LastUpdated is the timestamp when the gateway was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.gateway.provider"
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
