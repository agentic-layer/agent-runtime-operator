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

const (
	A2AProtocol = "A2A"
)

// AgentProtocol defines a port configuration for the agent
type AgentProtocol struct {
	// Name is the name of the port
	// +kubebuilder:default=``
	Name string `json:"name,omitempty"`

	// Type of the protocol used by the agent
	// +kubebuilder:validation:Enum=A2A
	Type string `json:"type"`

	// Port is the port number, defaults to the default port for the protocol
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Path is the path used for HTTP-based protocols
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_-]*$`
	Path string `json:"path,omitempty"`
}

// SubAgent defines configuration for connecting to either a cluster agent or remote agent
type SubAgent struct {
	// Name is a descriptive identifier for this sub-agent connection
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// AgentRef references an Agent resource in the cluster.
	// When specified, the operator will resolve the Agent's service URL automatically.
	// Only Name and Namespace fields are used; other fields (Kind, APIVersion, etc.) are ignored.
	// If Namespace is not specified, defaults to the same namespace as the current Agent.
	// Mutually exclusive with Url - exactly one must be specified.
	// +optional
	AgentRef *corev1.ObjectReference `json:"agentRef,omitempty"`

	// Url is the HTTP/HTTPS endpoint URL for a remote agent outside the cluster.
	// It refers to the agent's well-known agent card URL, e.g. https://agent.example.com/.well-known/agent-card.json
	// Mutually exclusive with AgentRef - exactly one must be specified.
	// +optional
	// +kubebuilder:validation:Format=uri
	Url string `json:"url,omitempty"`

	// InteractionType specifies how the agent should interact with this sub-agent.
	// +optional
	// +kubebuilder:validation:Enum=transfer;tool_call
	// +kubebuilder:default="tool_call"
	InteractionType string `json:"interactionType,omitempty"`
}

// AgentTool defines configuration for integrating an MCP (Model Context Protocol) tool
type AgentTool struct {
	// Name is the unique identifier for this tool
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ToolServerRef references a ToolServer resource in the cluster.
	// The operator will resolve the ToolServer's service URL automatically.
	// Only Name and Namespace fields are used; other fields (Kind, APIVersion, etc.) are ignored.
	// If Namespace is not specified, defaults to the same namespace as the current Agent.
	// Mutually exclusive with Url - exactly one must be specified.
	// +optional
	ToolServerRef *corev1.ObjectReference `json:"toolServerRef,omitempty"`

	// Url is the HTTP/HTTPS endpoint URL for an MCP tool server outside the cluster.
	// Mutually exclusive with ToolServerRef - exactly one must be specified.
	// +optional
	// +kubebuilder:validation:Format=uri
	Url string `json:"url,omitempty"`
}

// AgentSpec defines the desired state of Agent.
type AgentSpec struct {
	// Framework defines the supported agent frameworks
	// +kubebuilder:validation:Enum=google-adk;custom
	// +optional
	Framework string `json:"framework,omitempty"`

	// Replicas is the number of replicas for the microservice deployment
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Image is the Docker image and tag to use for the microservice deployment.
	// When not specified, the operator will use a framework-specific template image.
	// +optional
	Image string `json:"image,omitempty"`

	// Description provides a description of the agent.
	// This is passed as AGENT_DESCRIPTION environment variable to the agent.
	// +optional
	Description string `json:"description,omitempty"`

	// Instruction defines the system instruction/prompt for the agent when using template images.
	// This is passed as AGENT_INSTRUCTION environment variable to the agent.
	// +optional
	Instruction string `json:"instruction,omitempty"`

	// Model specifies the language model to use for the agent.
	// This is passed as AGENT_MODEL environment variable to the agent.
	// Defaults to the agents default model if not specified.
	// +optional
	Model string `json:"model,omitempty"`

	// SubAgents defines configuration for connecting to cluster or remote agents.
	// This is converted to JSON and passed as SUB_AGENTS environment variable to the agent.
	// +optional
	SubAgents []SubAgent `json:"subAgents,omitempty"`

	// Tools defines configuration for integrating MCP (Model Context Protocol) tools.
	// This is converted to JSON and passed as AGENT_TOOLS environment variable to the agent.
	// +optional
	Tools []AgentTool `json:"tools,omitempty"`

	// Protocols defines the protocols supported by the agent
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=atomic
	Protocols []AgentProtocol `json:"protocols,omitempty"`

	// Exposed indicates whether this agent should be exposed via the AgentGateway
	// +kubebuilder:default=false
	Exposed bool `json:"exposed,omitempty"`

	// AiGatewayRef references an AiGateway resource that this agent should use for model routing.
	// If not specified, the operator will attempt to find the default AiGateway in the cluster.
	// If no default AiGateway exists, the agent will run without an AI Gateway.
	// If Namespace is not specified, defaults to the same namespace as the Agent.
	// +optional
	AiGatewayRef *corev1.ObjectReference `json:"aiGatewayRef,omitempty"`

	// Env defines additional environment variables to be injected into the agent container.
	// These are take precedence over operator-managed environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom defines sources to populate environment variables from.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// VolumeMounts defines volume mounts to be added to the agent container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Volumes defines volumes to be added to the agent pod.
	// Volume names starting with "agent-operator-" are reserved for operator use and will be rejected.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Resources defines the compute resource requirements for the agent container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// AgentStatus defines the observed state of Agent.
type AgentStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// Url is the cluster-local URL where this agent can be accessed via A2A protocol.
	// This is automatically populated by the controller when the agent has an A2A protocol configured.
	// Format: http://{name}.{namespace}.svc.cluster.local:{port}/.well-known/agent-card.json
	// +optional
	Url string `json:"url,omitempty"`

	// AiGatewayRef references the AiGateway resource that this agent is connected to.
	// This field is automatically populated by the controller when an AI Gateway is being used.
	// If nil, the agent is not connected to any AI Gateway.
	// +optional
	AiGatewayRef *corev1.ObjectReference `json:"aiGatewayRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AI Gateway",type=string,JSONPath=".status.aiGatewayRef.name"

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
