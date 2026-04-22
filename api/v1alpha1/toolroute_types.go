/*
Copyright 2025 Agentic Layer.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToolRouteSpec defines the desired state of ToolRoute.
type ToolRouteSpec struct {
	// ToolGatewayRef identifies the ToolGateway hosting this route.
	// Namespace defaults to the ToolRoute's namespace if not specified.
	// +kubebuilder:validation:Required
	ToolGatewayRef corev1.ObjectReference `json:"toolGatewayRef"`

	// Upstream specifies the MCP server this route proxies.
	// Exactly one of ToolServerRef or External must be set.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="(has(self.toolServerRef) ? 1 : 0) + (has(self.external) ? 1 : 0) == 1",message="exactly one of toolServerRef or external must be set"
	Upstream ToolRouteUpstream `json:"upstream"`

	// ToolFilter restricts which tools are exposed through this route.
	// If nil, all tools pass through unfiltered.
	// +optional
	ToolFilter *ToolFilter `json:"toolFilter,omitempty"`
}

// ToolRouteUpstream describes the upstream MCP server for a route.
// Exactly one of ToolServerRef or External must be set.
type ToolRouteUpstream struct {
	// ToolServerRef references a cluster-local ToolServer.
	// Namespace defaults to the ToolRoute's namespace if not specified.
	// Mutually exclusive with External.
	// +optional
	ToolServerRef *corev1.ObjectReference `json:"toolServerRef,omitempty"`

	// External describes a remote MCP server reachable at an HTTP URL.
	// Mutually exclusive with ToolServerRef.
	// +optional
	External *ExternalUpstream `json:"external,omitempty"`
}

// ExternalUpstream describes a remote MCP server reachable at an HTTP URL.
type ExternalUpstream struct {
	// Url is the HTTP/HTTPS endpoint of the external MCP server.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	Url string `json:"url"`
}

// ToolFilter restricts which tools are exposed through a route.
// Matching uses glob syntax: "*" matches any run of characters, "?" matches one.
// Deny is applied after Allow and wins on conflict.
type ToolFilter struct {
	// Allow is an allowlist of tool names and glob patterns.
	// If non-empty, only matching tools are exposed.
	// If empty or nil, all tools are candidates (subject to Deny).
	// +optional
	Allow []string `json:"allow,omitempty"`

	// Deny is a denylist of tool names and glob patterns.
	// Applied after Allow; Deny wins on conflict.
	// +optional
	Deny []string `json:"deny,omitempty"`
}

// ToolRouteStatus defines the observed state of ToolRoute.
type ToolRouteStatus struct {
	// Url is the reachable URL assigned by the gateway implementation.
	// Agents consume this URL verbatim.
	// +optional
	Url string `json:"url,omitempty"`

	// Conditions represent the latest available observations of the route's state.
	// Known condition types: Accepted, Ready, ResolutionFailed.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Gateway",type="string",JSONPath=".spec.toolGatewayRef.name"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ToolRoute is the Schema for the toolroutes API.
type ToolRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ToolRouteSpec   `json:"spec,omitempty"`
	Status ToolRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolRouteList contains a list of ToolRoute.
type ToolRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ToolRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ToolRoute{}, &ToolRouteList{})
}
