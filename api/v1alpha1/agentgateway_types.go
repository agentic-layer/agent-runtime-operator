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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentGatewaySpec defines the desired state of AgentGateway
type AgentGatewaySpec struct {
	// AgentGatewayClassName specifies which AgentGatewayClass to use for this gateway instance.
	// This is only needed if multiple gateway classes are defined in the cluster.
	AgentGatewayClassName string `json:"agentGatewayClassName,omitempty"`

	// Replicas is the number of gateway replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Timeout specifies the gateway timeout for requests
	// +kubebuilder:default="360s"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Environment variables to pass to the AgentGateway container.
	// These can include configuration values, credentials, or feature flags.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// List of sources to populate environment variables in the AgentGateway container.
	// This allows loading variables from ConfigMaps and Secrets.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// AgentGatewayStatus defines the observed state of AgentGateway
type AgentGatewayStatus struct {
	// Conditions represent the latest available observations of the gateway's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

// AgentGatewayClassSpec defines the desired state of AgentGatewayClass
type AgentGatewayClassSpec struct {
	// Controller is the name of the controller that should handle this gateway class
	// +kubebuilder:validation:Required
	Controller string `json:"controller"`
}

// AgentGatewayClassStatus defines the observed state of AgentGatewayClass
type AgentGatewayClassStatus struct {
	// Conditions represent the latest available observations of the gateway class's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Controller",type="string",JSONPath=".spec.controller"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AgentGatewayClass is the Schema for the agentgatewayclasses API
type AgentGatewayClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentGatewayClassSpec   `json:"spec,omitempty"`
	Status AgentGatewayClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentGatewayClassList contains a list of AgentGatewayClass
type AgentGatewayClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentGatewayClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentGateway{}, &AgentGatewayList{}, &AgentGatewayClass{}, &AgentGatewayClassList{})
}
