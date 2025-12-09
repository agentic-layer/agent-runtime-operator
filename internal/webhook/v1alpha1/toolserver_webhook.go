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
	"context"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	mcpProtocol     = "mcp"
	httpTransport   = "http"
	sseTransport    = "sse"
	defaultPort     = int32(8080)
	defaultReplicas = int32(1)
	defaultHttpPath = "/mcp"
	defaultSsePath  = "/sse"
)

// log is for logging in this package.
var toolserverlog = logf.Log.WithName("toolserver-resource")

// SetupToolServerWebhookWithManager registers the webhook for ToolServer in the manager.
func SetupToolServerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.ToolServer{}).
		WithValidator(&ToolServerCustomValidator{}).
		WithDefaulter(&ToolServerCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-runtime-agentic-layer-ai-v1alpha1-toolserver,mutating=true,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=toolservers,verbs=create;update,versions=v1alpha1,name=mtoolserver-v1alpha1.kb.io,admissionReviewVersions=v1

// ToolServerCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ToolServer when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ToolServerCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &ToolServerCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ToolServer.
func (d *ToolServerCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	toolserver, ok := obj.(*runtimev1alpha1.ToolServer)

	if !ok {
		return fmt.Errorf("expected an ToolServer object but got %T", obj)
	}
	toolserverlog.Info("Defaulting for ToolServer", "name", toolserver.GetName())

	d.applyDefaults(toolserver)

	return nil
}

// applyDefaults applies default values to the ToolServer
func (d *ToolServerCustomDefaulter) applyDefaults(toolserver *runtimev1alpha1.ToolServer) {
	// Set default protocol if not specified
	if toolserver.Spec.Protocol == "" {
		toolserver.Spec.Protocol = mcpProtocol
	}

	// Set default port for http/sse transports if not specified
	if (toolserver.Spec.TransportType == httpTransport || toolserver.Spec.TransportType == sseTransport) &&
		toolserver.Spec.Port == 0 {
		toolserver.Spec.Port = defaultPort
	}

	// Set default path for http/sse transports if not specified
	if toolserver.Spec.Path == "" {
		switch toolserver.Spec.TransportType {
		case httpTransport:
			toolserver.Spec.Path = defaultHttpPath
		case sseTransport:
			toolserver.Spec.Path = defaultSsePath
		}
	}

	// Set default replicas for http/sse transports if not specified
	if (toolserver.Spec.TransportType == httpTransport || toolserver.Spec.TransportType == sseTransport) &&
		toolserver.Spec.Replicas == nil {
		replicas := defaultReplicas
		toolserver.Spec.Replicas = &replicas
	}
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-runtime-agentic-layer-ai-v1alpha1-toolserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=toolservers,verbs=create;update,versions=v1alpha1,name=vtoolserver-v1alpha1.kb.io,admissionReviewVersions=v1

// ToolServerCustomValidator struct is responsible for validating the ToolServer resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ToolServerCustomValidator struct{}

var _ webhook.CustomValidator = &ToolServerCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ToolServer.
func (v *ToolServerCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	toolserver, ok := obj.(*runtimev1alpha1.ToolServer)
	if !ok {
		return nil, fmt.Errorf("expected a ToolServer object but got %T", obj)
	}
	toolserverlog.Info("Validation for ToolServer upon creation", "name", toolserver.GetName())

	return v.validateToolServer(toolserver)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ToolServer.
func (v *ToolServerCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	toolserver, ok := newObj.(*runtimev1alpha1.ToolServer)
	if !ok {
		return nil, fmt.Errorf("expected a ToolServer object for the newObj but got %T", newObj)
	}
	toolserverlog.Info("Validation for ToolServer upon update", "name", toolserver.GetName())

	return v.validateToolServer(toolserver)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ToolServer.
func (v *ToolServerCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// No validation needed on delete
	return nil, nil
}

// validateToolServer performs validation logic for ToolServer resources
func (v *ToolServerCustomValidator) validateToolServer(toolserver *runtimev1alpha1.ToolServer) (admission.Warnings, error) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// http/sse MUST have port set (after defaults)
	if toolserver.Spec.Port == 0 {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "port"),
			fmt.Sprintf("port is required for %s transport", toolserver.Spec.TransportType),
		))
	}

	// Validate path format if specified
	if toolserver.Spec.Path != "" && toolserver.Spec.Path != "/" && !isValidPathFormat(toolserver.Spec.Path) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "path"),
			toolserver.Spec.Path,
			"path must start with '/' and contain only valid URL path characters",
		))
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}

	return warnings, nil
}

// isValidPathFormat validates URL path format
func isValidPathFormat(path string) bool {
	// Basic validation - path must start with /
	if len(path) == 0 || path[0] != '/' {
		return false
	}

	// Check for invalid characters that could cause issues
	// Spaces, tabs, newlines, and control characters should not be in paths
	for _, r := range path {
		if r < 0x20 || r == 0x7F { // Control characters
			return false
		}
		if r == ' ' { // Space
			return false
		}
	}

	// Check for URL components that shouldn't be in a path field
	// (query strings and fragments should be separate)
	for _, invalid := range []string{"?", "#"} {
		if strings.Contains(path, invalid) {
			return false
		}
	}

	// Validate it's a valid URL path by parsing
	// We prepend a dummy scheme and host since paths alone aren't valid URLs
	testURL := "http://example.com" + path
	parsedURL, err := url.Parse(testURL)
	if err != nil {
		return false
	}

	// Verify the path wasn't modified during parsing
	// (this catches encoding issues or invalid sequences)
	if parsedURL.Path != path {
		return false
	}

	return true
}
