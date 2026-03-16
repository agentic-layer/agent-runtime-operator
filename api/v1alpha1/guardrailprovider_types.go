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

// GuardrailProviderSpec defines the desired state of GuardrailProvider.
type GuardrailProviderSpec struct {
	// Protocol defines the guardrail protocol used by this provider.
	// +kubebuilder:validation:Enum=openai-moderation;bedrock
	Protocol string `json:"protocol"`

	// ApiKeySecretRef references a Kubernetes Secret containing the API key for the guardrail provider.
	// The secret must contain the key specified in the SecretKeySelector.
	// +optional
	ApiKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`

	// TransportType defines the transport used to communicate with the guardrail backend.
	// Required when BackendRef is specified.
	// +kubebuilder:validation:Enum=http;grpc;envoy-ext-proc
	// +optional
	TransportType string `json:"transportType,omitempty"`

	// BackendRef references the Kubernetes Service acting as the guardrail backend.
	// When omitted, the provider uses the protocol's default managed endpoint
	// (e.g., the official OpenAI moderation API or AWS Bedrock).
	// Mutually exclusive with ExternalUrl.
	// +optional
	BackendRef *GuardrailBackendRef `json:"backendRef,omitempty"`

	// ExternalUrl specifies an external URL for the guardrail backend.
	// Use this to point to an external guardrail service outside the cluster.
	// Mutually exclusive with BackendRef.
	// +optional
	// +kubebuilder:validation:Format=uri
	ExternalUrl string `json:"externalUrl,omitempty"`
}

// GuardrailBackendRef references a Kubernetes Service acting as the guardrail backend.
type GuardrailBackendRef struct {
	corev1.ObjectReference `json:",inline"`

	// Port is the port number of the Kubernetes Service.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// GuardrailProviderStatus defines the observed state of GuardrailProvider.
type GuardrailProviderStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GuardrailProvider is the Schema for the guardrailproviders API.
type GuardrailProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GuardrailProviderSpec   `json:"spec,omitempty"`
	Status GuardrailProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GuardrailProviderList contains a list of GuardrailProvider.
type GuardrailProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GuardrailProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GuardrailProvider{}, &GuardrailProviderList{})
}
