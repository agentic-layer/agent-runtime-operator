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

// GuardSpec defines the desired state of Guard.
type GuardSpec struct {
	// Name is the identifier of the guard as known by the referenced GuardrailProvider.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Version is the version of the guard at the provider (if supported).
	// +optional
	Version string `json:"version,omitempty"`

	// Mode defines when the guard is applied relative to the LLM call.
	// +kubebuilder:validation:Enum=pre_call;post_call;during_call
	Mode string `json:"mode"`

	// Description provides a human-readable description of the guard's purpose.
	// This field is for documentation purposes only and has no effect on the guard's behavior.
	// +optional
	Description string `json:"description,omitempty"`

	// ProviderRef references the GuardrailProvider that hosts this guard.
	// If Namespace is not specified, defaults to the same namespace as the Guard.
	ProviderRef GuardrailProviderRef `json:"providerRef"`
}

// GuardrailProviderRef is a reference to a GuardrailProvider resource.
type GuardrailProviderRef struct {
	// Name is the name of the GuardrailProvider.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace is the namespace of the GuardrailProvider.
	// If not specified, defaults to the same namespace as the Guard.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// GuardStatus defines the observed state of Guard.
type GuardStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Guard is the Schema for the guards API.
type Guard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GuardSpec   `json:"spec,omitempty"`
	Status GuardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GuardList contains a list of Guard.
type GuardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Guard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Guard{}, &GuardList{})
}
