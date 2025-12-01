/*
Copyright 2025 Agentic Layer.

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

// AiGatewaySpec defines the desired state of AiGateway.
type AiGatewaySpec struct {
	// AiGatewayClassName specifies which AiGatewayClass to use for this AI gateway instance.
	// This is only needed if multiple AI gateway classes are defined in the cluster.
	AiGatewayClassName string `json:"aiGatewayClassName,omitempty"`

	// Port on which the AI gateway will be exposed.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=4000
	Port int32 `json:"port,omitempty"`

	// List of AI models to be made available through the gateway.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	AiModels []AiModel `json:"aiModels,omitempty"`

	// Environment variables to pass to the AI gateway container.
	// These can include configuration values, credentials, or feature flags.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// List of sources to populate environment variables in the AI gateway container.
	// This allows loading variables from ConfigMaps and Secrets.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

type AiModel struct {
	// Name is the identifier for the AI model (e.g., "gpt-4", "claude-3-opus")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Provider specifies the AI provider (e.g., "openai", "anthropic", "azure")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`
}

// AiGatewayStatus defines the observed state of AiGateway.
type AiGatewayStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AiGateway is the Schema for the AI gateways API.
type AiGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AiGatewaySpec   `json:"spec,omitempty"`
	Status AiGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AiGatewayList contains a list of AiGateway.
type AiGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AiGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AiGateway{}, &AiGatewayList{})
}
