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

// AgentProtocol defines a port configuration for the agent
type AgentProtocol struct {
	// Name is the name of the port
	Name string `json:"name,omitempty"`

	// Type of the protocol used by the agent
	// +kubebuilder:validation:Enum=A2A;OpenAI
	Type string `json:"type"`

	// Port is the port number, defaults to the default port for the protocol
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_-]*$`
	// Path is the path used for HTTP-based protocols
	Path string `json:"path,omitempty"`
}

// SubAgent defines configuration for connecting to a remote agent
type SubAgent struct {
	// Name is the unique identifier for this sub-agent
	Name string `json:"name"`

	// Url is the HTTP/HTTPS endpoint URL for the remote agent configuration.
	// Supports both HTTP (for internal cluster URLs) and HTTPS schemes.
	// +kubebuilder:validation:Pattern=`^https?://[a-zA-Z0-9.-]+(/.*)?$`
	// +kubebuilder:validation:Format=uri
	Url string `json:"url"`
}

// AgentTool defines configuration for integrating an MCP (Model Context Protocol) tool
type AgentTool struct {
	// Name is the unique identifier for this tool
	Name string `json:"name"`

	// Url is the HTTP/HTTPS endpoint URL for the MCP tool server.
	// Supports both HTTP (for internal cluster URLs) and HTTPS schemes.
	// +kubebuilder:validation:Pattern=`^https?://[a-zA-Z0-9.-]+(/.*)?$`
	// +kubebuilder:validation:Format=uri
	Url string `json:"url"`
}

// AgentSpec defines the desired state of Agent.
type AgentSpec struct {
	// +optional
	// Framework defines the supported agent frameworks
	// +kubebuilder:validation:Enum=google-adk;custom
	Framework string `json:"framework,omitempty"`

	// +optional
	// Replicas is the number of replicas for the microservice deployment
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// +optional
	// Image is the Docker image and tag to use for the microservice deployment.
	// When not specified, the operator will use a framework-specific template image.
	Image string `json:"image,omitempty"`

	// +optional
	// Description provides a description of the agent.
	// This is passed as AGENT_DESCRIPTION environment variable to the agent.
	Description string `json:"description,omitempty"`

	// +optional
	// Instruction defines the system instruction/prompt for the agent when using template images.
	// This is passed as AGENT_INSTRUCTION environment variable to the agent.
	Instruction string `json:"instruction,omitempty"`

	// +optional
	// Model specifies the language model to use for the agent.
	// This is passed as AGENT_MODEL environment variable to the agent.
	// Defaults to "gemini/gemini-2.0-flash" if not specified.
	Model string `json:"model,omitempty"`

	// +optional
	// SubAgents defines configuration for connecting to remote agents.
	// This is converted to JSON and passed as SUB_AGENTS environment variable to the agent.
	SubAgents []SubAgent `json:"subAgents,omitempty"`

	// +optional
	// Tools defines configuration for integrating MCP (Model Context Protocol) tools.
	// This is converted to JSON and passed as AGENT_TOOLS environment variable to the agent.
	Tools []AgentTool `json:"tools,omitempty"`

	// Protocols defines the protocols supported by the agent
	Protocols []AgentProtocol `json:"protocols,omitempty"`

	// +optional
	// Env defines additional environment variables to be injected into the agent container.
	// These are take precedence over operator-managed environment variables.
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	// EnvFrom defines sources to populate environment variables from.
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// AgentStatus defines the observed state of Agent.
type AgentStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Agent is the Schema for the agents API.
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentList contains a list of Agent.
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
