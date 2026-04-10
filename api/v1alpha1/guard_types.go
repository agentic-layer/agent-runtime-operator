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

// GuardMode defines when a guard is applied relative to the LLM call.
// +kubebuilder:validation:Enum=pre_call;post_call;during_call
type GuardMode string

const (
	GuardModePreCall    GuardMode = "pre_call"
	GuardModePostCall   GuardMode = "post_call"
	GuardModeDuringCall GuardMode = "during_call"
)

// GuardSpec defines the desired state of Guard.
type GuardSpec struct {
	// Mode defines when the guard is applied relative to the LLM call.
	// Multiple modes can be specified to apply the guard at multiple points.
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	Mode []GuardMode `json:"mode"`

	// Description provides a human-readable description of the guard's purpose.
	// This field is for documentation purposes only and has no effect on the guard's behavior.
	// +optional
	Description string `json:"description,omitempty"`

	// ProviderRef references the GuardrailProvider that hosts this guard.
	// If Namespace is not specified, defaults to the same namespace as the Guard.
	ProviderRef corev1.ObjectReference `json:"providerRef"`

	// OpenAIModeration holds guard-level configuration for the OpenAI Moderation API.
	// +optional
	OpenAIModeration *OpenAIModerationGuardConfig `json:"openaiModeration,omitempty"`

	// Bedrock holds guard-level configuration for the AWS Bedrock Guardrails API.
	// +optional
	Bedrock *BedrockGuardConfig `json:"bedrock,omitempty"`

	// Presidio holds guard-level configuration for the Presidio API.
	// +optional
	Presidio *PresidioGuardConfig `json:"presidio,omitempty"`
}

// OpenAIModerationGuardConfig holds guard-level configuration for the OpenAI Moderation API.
type OpenAIModerationGuardConfig struct {
	// Model specifies the moderation model to use (e.g., "omni-moderation-latest").
	// When omitted, the provider's default model is used.
	// +optional
	Model string `json:"model,omitempty"`
}

// BedrockGuardConfig holds guard-level configuration for the AWS Bedrock Guardrails API.
type BedrockGuardConfig struct {
	// GuardrailId is the identifier of the Bedrock guardrail.
	GuardrailId string `json:"guardrailId"`

	// GuardrailVersion is the version of the Bedrock guardrail.
	// +optional
	GuardrailVersion string `json:"guardrailVersion,omitempty"`
}

// PresidioGuardConfig holds guard-level configuration for the Presidio API.
//
// For a full list of supported entity types, see:
// https://microsoft.github.io/presidio/supported_entities/
//
// Common entity types include:
//   - PERSON, EMAIL_ADDRESS, PHONE_NUMBER, CREDIT_CARD
//   - IP_ADDRESS, URL, IBAN_CODE, LOCATION, DATE_TIME
//   - US_SSN, US_PASSPORT, US_DRIVER_LICENSE, US_BANK_NUMBER
//   - UK_NHS, UK_NINO, UK_PASSPORT
type PresidioGuardConfig struct {
	// Language specifies the language of the text to analyze (e.g., "en").
	// Defaults to "en" when omitted.
	// +optional
	Language string `json:"language,omitempty"`

	// ScoreThresholds maps entity types to minimum confidence scores (0.0 to 1.0).
	// Use "ALL" as key to set a default threshold for all entity types.
	// Per-entity thresholds override the "ALL" key.
	// Example: {"ALL": "0.5", "EMAIL_ADDRESS": "0.8", "PHONE_NUMBER": "0.6"}
	// +optional
	ScoreThresholds map[string]string `json:"scoreThresholds,omitempty"`

	// EntityActions maps PII entity types to actions to take when detected.
	// Supported actions are "MASK" (redact the entity) and "BLOCK" (reject the request).
	// When omitted, detected entities are blocked by default.
	// The keys also determine which entity types are detected.
	// When omitted entirely, all supported entities are detected.
	// Example: {"EMAIL_ADDRESS": "MASK", "CREDIT_CARD": "BLOCK", "PHONE_NUMBER": "MASK"}
	// +optional
	EntityActions map[string]string `json:"entityActions,omitempty"`
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
