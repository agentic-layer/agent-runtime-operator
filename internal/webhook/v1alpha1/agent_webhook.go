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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	googleAdkFramework           = "google-adk"
	DefaultTemplateImageAdk      = "ghcr.io/agentic-layer/agent-template-adk:0.2.0"
	defaultTemplateImageFallback = "invalid"
)

// nolint:unused
// log is for logging in this package.
var agentlog = logf.Log.WithName("agent-resource")

// SetupAgentWebhookWithManager registers the webhook for Agent in the manager.
func SetupAgentWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.Agent{}).
		WithDefaulter(&AgentCustomDefaulter{
			DefaultFramework:     googleAdkFramework,
			DefaultReplicas:      1,
			DefaultPort:          8080,
			DefaultPortGoogleAdk: 8000,
			Recorder:             mgr.GetEventRecorderFor("agent-defaulter-webhook"),
		}).
		WithValidator(&AgentCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-runtime-agentic-layer-ai-v1alpha1-agent,mutating=true,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agents,verbs=create;update,versions=v1alpha1,name=magent-v1alpha1.kb.io,admissionReviewVersions=v1

// AgentCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Agent when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AgentCustomDefaulter struct {
	DefaultFramework     string
	DefaultReplicas      int32
	DefaultPort          int32
	DefaultPortGoogleAdk int32
	Recorder             record.EventRecorder
}

var _ webhook.CustomDefaulter = &AgentCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Agent.
func (d *AgentCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	agent, ok := obj.(*runtimev1alpha1.Agent)

	if !ok {
		return fmt.Errorf("expected an Agent object but got %T", obj)
	}
	agentlog.Info("Defaulting for Agent", "name", agent.GetName())

	d.applyDefaults(agent)

	return nil
}

// applyDefaults applies default values to the Agent.
func (d *AgentCustomDefaulter) applyDefaults(agent *runtimev1alpha1.Agent) {
	// Set default framework if not specified
	if agent.Spec.Framework == "" {
		agent.Spec.Framework = d.DefaultFramework
	}

	// Set default replicas if not specified
	if agent.Spec.Replicas == nil {
		agent.Spec.Replicas = new(int32)
		*agent.Spec.Replicas = d.DefaultReplicas
	}

	// Set default image if not specified (template image)
	if agent.Spec.Image == "" {
		switch agent.Spec.Framework {
		case googleAdkFramework:
			agent.Spec.Image = DefaultTemplateImageAdk
		default:
			// Validation will catch unsupported frameworks without images
			// This shouldn't be reached due to validation, but set template as fallback
			agent.Spec.Image = defaultTemplateImageFallback
		}
	}

	// Set default ports for protocols if not specified
	for i, protocol := range agent.Spec.Protocols {
		if protocol.Port == 0 {
			agent.Spec.Protocols[i].Port = d.frameworkDefaultPort(agent.Spec.Framework)
		}
		if protocol.Name == "" {
			agent.Spec.Protocols[i].Name = fmt.Sprintf("%s-%d", sanitizeForPortName(protocol.Type), agent.Spec.Protocols[i].Port)
		}
	}
}

func (d *AgentCustomDefaulter) frameworkDefaultPort(framework string) int32 {
	switch framework {
	case googleAdkFramework:
		return d.DefaultPortGoogleAdk
	default:
		return d.DefaultPort // Default port for unknown frameworks
	}
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-runtime-agentic-layer-ai-v1alpha1-agent,mutating=false,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agents,verbs=create;update,versions=v1alpha1,name=vagent-v1alpha1.kb.io,admissionReviewVersions=v1

// AgentCustomValidator struct is responsible for validating the Agent resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AgentCustomValidator struct{}

var _ webhook.CustomValidator = &AgentCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the Kind Agent.
func (v *AgentCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	agent, ok := obj.(*runtimev1alpha1.Agent)
	if !ok {
		return nil, fmt.Errorf("expected an Agent object but got %T", obj)
	}
	agentlog.Info("Validating Agent on create", "name", agent.GetName())
	return v.validateAgent(agent)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the Kind Agent.
func (v *AgentCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	agent, ok := newObj.(*runtimev1alpha1.Agent)
	if !ok {
		return nil, fmt.Errorf("expected an Agent object but got %T", newObj)
	}
	agentlog.Info("Validating Agent on update", "name", agent.GetName())
	return v.validateAgent(agent)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the Kind Agent.
func (v *AgentCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// No validation needed on delete
	return nil, nil
}

// validateAgent performs validation logic for Agent resources.
func (v *AgentCustomValidator) validateAgent(agent *runtimev1alpha1.Agent) (admission.Warnings, error) {
	var allErrs field.ErrorList

	// Validate framework and image combination
	if agent.Spec.Image == "" && agent.Spec.Framework != googleAdkFramework {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "framework"),
			agent.Spec.Framework,
			fmt.Sprintf("framework %q requires a custom image. Template agents are only supported for %q framework", agent.Spec.Framework, googleAdkFramework),
		))
	}

	// Validate SubAgent URLs
	for i, subAgent := range agent.Spec.SubAgents {
		if err := v.validateURL(subAgent.Url, fmt.Sprintf("SubAgent[%d].Url", i)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "subAgents").Index(i).Child("url"),
				subAgent.Url,
				err.Error(),
			))
		}
	}

	// Validate Tool URLs
	for i, tool := range agent.Spec.Tools {
		if err := v.validateURL(tool.Url, fmt.Sprintf("Tool[%d].Url", i)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "tools").Index(i).Child("url"),
				tool.Url,
				err.Error(),
			))
		}
	}

	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	return nil, nil
}

// validateURL validates that a URL is properly formatted and uses HTTP or HTTPS scheme.
// Both schemes are allowed to support external HTTPS URLs and internal cluster HTTP URLs.
func (v *AgentCustomValidator) validateURL(urlStr, fieldName string) error {
	if urlStr == "" {
		return nil // Empty URLs are allowed (optional fields)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %v", fieldName, err)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("%s must use HTTP or HTTPS scheme, got %q", fieldName, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s must have a valid host", fieldName)
	}

	return nil
}

// sanitizeForPortName converts a string to be valid for Kubernetes port names.
// Port names must contain only lowercase letters, numbers, and hyphens,
// and must start and end with an alphanumeric character.
func sanitizeForPortName(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)

	// Replace any character that's not a lowercase letter, digit, or hyphen with a hyphen
	var sanitized strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sanitized.WriteRune(r)
		} else {
			sanitized.WriteRune('-')
		}
	}

	result = sanitized.String()

	// Remove leading and trailing hyphens and ensure it starts/ends with alphanumeric
	result = strings.Trim(result, "-")

	// Ensure we have a valid result
	if result == "" {
		result = "port"
	}

	return result
}
