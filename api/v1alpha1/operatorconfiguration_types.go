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

// OperatorConfigurationSpec defines the desired state of OperatorConfiguration.
type OperatorConfigurationSpec struct {
	// AgentTemplateImages defines the default template images for different agent frameworks.
	// These images are used when an Agent resource does not specify a custom image.
	// +optional
	AgentTemplateImages *AgentTemplateImages `json:"agentTemplateImages,omitempty"`
}

// AgentTemplateImages defines template images for agent frameworks.
type AgentTemplateImages struct {
	// GoogleAdk is the template image for the Google ADK framework.
	// If not specified, the operator's built-in default will be used.
	// +optional
	GoogleAdk string `json:"googleAdk,omitempty"`

	// Additional framework images can be added here in the future, e.g.:
	// LangChain string `json:"langChain,omitempty"`
	// CrewAI string `json:"crewAi,omitempty"`
}

// OperatorConfigurationStatus defines the observed state of OperatorConfiguration.
type OperatorConfigurationStatus struct {
	// Conditions represent the latest available observations of the configuration's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Google ADK Image",type=string,JSONPath=`.spec.agentTemplateImages.googleAdk`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OperatorConfiguration is the Schema for the operatorconfigurations API.
// It provides cluster-wide configuration for the agent-runtime-operator.
type OperatorConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperatorConfigurationSpec   `json:"spec,omitempty"`
	Status OperatorConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperatorConfigurationList contains a list of OperatorConfiguration.
type OperatorConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperatorConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperatorConfiguration{}, &OperatorConfigurationList{})
}
