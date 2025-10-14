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

// ToolServerSpec defines the desired state of ToolServer.
type ToolServerSpec struct {
	// Protocol defines the tool server protocol (e.g., "mcp" for Model Context Protocol)
	// +kubebuilder:validation:Enum=mcp
	Protocol string `json:"protocol"`

	// TransportType defines how the tool server communicates
	// - stdio: For sidecar injection into agent pods (no standalone deployment)
	// - http: HTTP transport with standalone deployment and service
	// - sse: Server-Sent Events transport with standalone deployment and service
	// +kubebuilder:validation:Enum=stdio;http;sse
	TransportType string `json:"transportType"`

	// Image is the container image for the tool server
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Command overrides the container's ENTRYPOINT.
	// If not specified, the container image's ENTRYPOINT is used.
	// Follows the same semantics as Kubernetes Pod containers.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args overrides the container's CMD.
	// If not specified, the container image's CMD is used.
	// Follows the same semantics as Kubernetes Pod containers.
	// +optional
	Args []string `json:"args,omitempty"`

	// Port is the port number for http/sse transports
	// Required for http and sse transports, must not be set for stdio
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Path is the URL path for http/sse transports
	// Defaults to "/mcp" for http and "/sse" for sse if not specified
	// Must start with "/" if specified
	// Not applicable for stdio transport
	// +optional
	// +kubebuilder:validation:Pattern=`^/.*`
	Path string `json:"path,omitempty"`

	// Replicas is the number of replicas for http/sse transports
	// Only applicable for http and sse transports, ignored for stdio
	// +optional
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Env defines additional environment variables to be injected into the tool server container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom defines sources to populate environment variables from
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// ToolServerStatus defines the observed state of ToolServer.
type ToolServerStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// Url is the cluster-local URL where this tool server can be accessed
	// Only populated for http and sse transports
	// Format: http://{name}.{namespace}.svc.cluster.local:{port}{path}
	// +optional
	Url string `json:"url,omitempty"`
}

// ToolServer is the Schema for the toolservers API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ToolServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolServerSpec   `json:"spec,omitempty"`
	Status ToolServerStatus `json:"status,omitempty"`
}

// ToolServerList contains a list of ToolServer.
//
// +kubebuilder:object:root=true
type ToolServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolServer{}, &ToolServerList{})
}
