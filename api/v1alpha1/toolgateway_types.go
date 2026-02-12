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

// ToolGatewaySpec defines the desired state of ToolGateway
type ToolGatewaySpec struct {
	// ToolGatewayClassName specifies which ToolGatewayClass to use for this gateway instance.
	// This is only needed if multiple gateway classes are defined in the cluster.
	ToolGatewayClassName string `json:"toolGatewayClassName,omitempty"`

	// Environment variables to pass to the ToolGateway container.
	// These can include configuration values, credentials, or feature flags.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// List of sources to populate environment variables in the ToolGateway container.
	// This allows loading variables from ConfigMaps and Secrets.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// ToolGatewayStatus defines the observed state of ToolGateway
type ToolGatewayStatus struct {
	// Conditions represent the latest available observations of the gateway's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ToolGateway is the Schema for the toolgateways API
type ToolGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolGatewaySpec   `json:"spec,omitempty"`
	Status ToolGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolGatewayList contains a list of ToolGateway
type ToolGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolGateway `json:"items"`
}

// ToolGatewayClassSpec defines the desired state of ToolGatewayClass
type ToolGatewayClassSpec struct {
	// Controller is the name of the controller that should handle this gateway class
	// +kubebuilder:validation:Required
	Controller string `json:"controller"`
}

// ToolGatewayClassStatus defines the observed state of ToolGatewayClass
type ToolGatewayClassStatus struct {
	// Conditions represent the latest available observations of the gateway class's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Controller",type="string",JSONPath=".spec.controller"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ToolGatewayClass is the Schema for the toolgatewayclasses API
type ToolGatewayClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolGatewayClassSpec   `json:"spec,omitempty"`
	Status ToolGatewayClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolGatewayClassList contains a list of ToolGatewayClass
type ToolGatewayClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolGatewayClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolGateway{}, &ToolGatewayList{}, &ToolGatewayClass{}, &ToolGatewayClassList{})
}
