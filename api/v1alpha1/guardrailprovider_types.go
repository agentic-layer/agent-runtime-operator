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
	// Type defines the guardrail API type implemented by this provider.
	// Each type corresponds to a specific guardrail API contract.
	// +kubebuilder:validation:Enum=openai-moderation-api;bedrock-api;presidio-api
	Type string `json:"type"`

	// OpenAIModeration holds configuration for providers implementing the OpenAI Moderation API.
	// Required when type is "openai-moderation-api".
	// +optional
	OpenAIModeration *OpenAIModerationProviderConfig `json:"openaiModeration,omitempty"`

	// Bedrock holds configuration for providers implementing the AWS Bedrock Guardrails API.
	// Required when type is "bedrock-api".
	// +optional
	Bedrock *BedrockProviderConfig `json:"bedrock,omitempty"`

	// Presidio holds configuration for providers implementing the Presidio API.
	// Required when type is "presidio-api".
	// +optional
	Presidio *PresidioProviderConfig `json:"presidio,omitempty"`
}

// OpenAIModerationProviderConfig holds configuration for the OpenAI Moderation API.
type OpenAIModerationProviderConfig struct {
	// BaseUrl overrides the default OpenAI moderation endpoint.
	// When omitted, the official OpenAI moderation API is used.
	// Can also point to a custom service implementing the same API contract.
	// +optional
	// +kubebuilder:validation:Format=uri
	BaseUrl string `json:"baseUrl,omitempty"`

	// ApiKeySecretRef references the secret containing the API key.
	// +optional
	ApiKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// BedrockProviderConfig holds configuration for the AWS Bedrock Guardrails API.
type BedrockProviderConfig struct {
	// Region is the AWS region for the Bedrock service.
	Region string `json:"region"`

	// CredentialsSecretRef references a secret containing AWS credentials.
	// The secret should contain "aws-access-key-id" and "aws-secret-access-key" keys.
	// When omitted, uses IRSA or pod identity for authentication.
	// +optional
	CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// PresidioProviderConfig holds configuration for the Presidio Analyzer API.
type PresidioProviderConfig struct {
	// BaseUrl is the Presidio analyzer service endpoint.
	// +kubebuilder:validation:Format=uri
	BaseUrl string `json:"baseUrl"`

	// ApiKeySecretRef references the secret containing the API key for the Presidio service.
	// +optional
	ApiKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
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
