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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AiGatewayClassSpec defines the desired state of AiGatewayClass.
type AiGatewayClassSpec struct {
	// Controller is the name of the controller that should handle this gateway class
	// +kubebuilder:validation:Required
	Controller string `json:"controller"`
}

// AiGatewayClassStatus defines the observed state of AiGatewayClass.
type AiGatewayClassStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AiGatewayClass is the Schema for the aigatewayclasses API.
type AiGatewayClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AiGatewayClassSpec   `json:"spec,omitempty"`
	Status AiGatewayClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AiGatewayClassList contains a list of AiGatewayClass.
type AiGatewayClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AiGatewayClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AiGatewayClass{}, &AiGatewayClassList{})
}
