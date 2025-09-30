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

// AgentReference defines a reference to an entry-point agent
type AgentReference struct {
	// Name is the name of the Agent resource
	Name string `json:"name"`

	// Namespace is the namespace of the Agent resource (optional, defaults to same namespace as AgenticWorkforce)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AgenticWorkforceSpec defines the desired state of AgenticWorkforce
type AgenticWorkforceSpec struct {
	// Name is the human-readable name of the workforce
	Name string `json:"name"`

	// Description provides details about the workforce's purpose and capabilities
	Description string `json:"description"`

	// Owner is the email address or identifier of the workforce owner
	Owner string `json:"owner"`

	// Tags is a list of labels for categorizing and organizing workforces
	// +optional
	Tags []string `json:"tags,omitempty"`

	// EntryPointAgents defines references to the entry-point agents for this workforce
	EntryPointAgents []AgentReference `json:"entryPointAgents"`
}

// AgenticWorkforceStatus defines the observed state of AgenticWorkforce
type AgenticWorkforceStatus struct {
	// Conditions represent the latest available observations of the workforce's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// TransitiveAgents contains all agents (both cluster and remote) discovered from entry-point agents
	// +optional
	TransitiveAgents []string `json:"transitiveAgents,omitempty"`

	// TransitiveTools contains all tools discovered from all transitive agents
	// +optional
	TransitiveTools []string `json:"transitiveTools,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Owner",type="string",JSONPath=".spec.owner"
// +kubebuilder:printcolumn:name="Entry Points",type="integer",JSONPath=".spec.entryPointAgents[*]"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AgenticWorkforce is the Schema for the agenticworkforces API
type AgenticWorkforce struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgenticWorkforceSpec   `json:"spec,omitempty"`
	Status AgenticWorkforceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgenticWorkforceList contains a list of AgenticWorkforce
type AgenticWorkforceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgenticWorkforce `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgenticWorkforce{}, &AgenticWorkforceList{})
}
