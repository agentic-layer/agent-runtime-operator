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
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
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
	googleAdkFramework      = "google-adk"
	DefaultTemplateImageAdk = "ghcr.io/agentic-layer/agent-template-adk:0.4.0"
)

// agentlog is for logging in this package.
var agentlog = logf.Log.WithName("agent-resource")

// AgentWebhookConfig contains configuration for the Agent webhook.
type AgentWebhookConfig struct {
	// AllowHostPath controls whether hostPath volumes are allowed in Agent resources.
	// When false (default), hostPath volumes will be rejected for security reasons.
	AllowHostPath bool
}

// SetupAgentWebhookWithManager registers the webhook for Agent in the manager.
func SetupAgentWebhookWithManager(mgr ctrl.Manager, config AgentWebhookConfig) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.Agent{}).
		WithDefaulter(&AgentCustomDefaulter{
			DefaultFramework:     googleAdkFramework,
			DefaultReplicas:      1,
			DefaultPort:          8080,
			DefaultPortGoogleAdk: 8000,
			Recorder:             mgr.GetEventRecorderFor("agent-defaulter-webhook"),
		}).
		WithValidator(&AgentCustomValidator{
			AllowHostPath: config.AllowHostPath,
		}).
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

	// Add default protocols if none are specified
	if len(agent.Spec.Protocols) == 0 {
		agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{{
			Type: runtimev1alpha1.A2AProtocol,
		}}
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
type AgentCustomValidator struct {
	// AllowHostPath controls whether hostPath volumes are allowed.
	AllowHostPath bool
}

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

	// Validate SubAgents
	for i, subAgent := range agent.Spec.SubAgents {
		if errs := v.validateSubAgent(subAgent, i); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Validate VolumeMounts reference existing Volumes
	if errs := v.validateVolumeMounts(agent); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate Volume sources
	for i, volume := range agent.Spec.Volumes {
		if errs := v.validateVolumeSource(volume, i); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	return nil, nil
}

// validateSubAgent validates a SubAgent configuration for business logic correctness.
// This performs stateless validation only - no cluster state checks.
// Basic URI format validation is handled by kubebuilder annotation.
func (v *AgentCustomValidator) validateSubAgent(subAgent runtimev1alpha1.SubAgent, index int) []*field.Error {
	var errs []*field.Error

	hasAgentRef := subAgent.AgentRef != nil
	hasURL := subAgent.Url != ""

	// Must have a name
	if subAgent.Name == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "subAgents").Index(index).Child("name"),
			"subAgent must have a name"))
	}

	// Exactly one of agentRef or url must be specified (mutually exclusive)
	if hasAgentRef && hasURL {
		errs = append(errs, field.Forbidden(
			field.NewPath("spec", "subAgents").Index(index),
			"agentRef and url are mutually exclusive - specify exactly one"))
	}

	if !hasAgentRef && !hasURL {
		errs = append(errs, field.Required(
			field.NewPath("spec", "subAgents").Index(index),
			"either agentRef or url must be specified"))
	}

	// Validate agentRef if provided
	if hasAgentRef && subAgent.AgentRef.Name == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "subAgents").Index(index).Child("agentRef", "name"),
			"agentRef.name must be specified"))
	}

	// Only validate scheme restriction (kubebuilder handles URI format)
	if hasURL {
		if err := v.validateHTTPScheme(subAgent.Url); err != nil {
			errs = append(errs, field.Invalid(
				field.NewPath("spec", "subAgents").Index(index).Child("url"),
				subAgent.Url,
				fmt.Sprintf("SubAgent[%d].Url: %s", index, err.Error())))
		}
	}

	return errs
}

// validateHTTPScheme validates that a URL uses HTTP or HTTPS scheme.
// URI format validation is handled by kubebuilder annotation.
func (v *AgentCustomValidator) validateHTTPScheme(urlStr string) error {
	if urlStr == "" {
		return nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// This shouldn't happen since kubebuilder validates URI format
		return fmt.Errorf("invalid URL format")
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("must use HTTP or HTTPS scheme, got %q", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("must have a valid host")
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

// validateVolumeMounts validates volume and volumeMount requirements.
// Performs additional validation that is not possible through OpenAPI validation by Kubernetes.
func (v *AgentCustomValidator) validateVolumeMounts(agent *runtimev1alpha1.Agent) []*field.Error {
	var errs []*field.Error

	// Build a set of available volume names for reference checking.
	volumeNames := make(map[string]bool)
	for _, volume := range agent.Spec.Volumes {
		volumeNames[volume.Name] = true
	}

	// Check each volumeMount references an existing volume (business logic validation)
	for i, volumeMount := range agent.Spec.VolumeMounts {

		if !volumeNames[volumeMount.Name] {
			errs = append(errs, field.Invalid(
				field.NewPath("spec", "volumeMounts").Index(i).Child("name"),
				volumeMount.Name,
				fmt.Sprintf("volumeMount references volume %q which does not exist in spec.volumes", volumeMount.Name)))
		}

		// Check for overlapping/nested mount paths (operator-specific enhanced validation)
		// Only check forward (j > i) to avoid reporting the same error multiple times
		for j := i + 1; j < len(agent.Spec.VolumeMounts); j++ {
			if agent.Spec.VolumeMounts[j].MountPath == "" {
				continue // Skip if empty (K8s will validate)
			}

			path1 := filepath.Clean(volumeMount.MountPath)
			path2 := filepath.Clean(agent.Spec.VolumeMounts[j].MountPath)

			// Skip exact duplicates (K8s validates duplicate mount paths)
			if path1 == path2 {
				continue
			}

			// Check if path2 is nested under path1
			// Add trailing slash to prevent false positives like /data and /data-backup
			if strings.HasPrefix(path2+"/", path1+"/") {
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "volumeMounts").Index(j).Child("mountPath"),
					path2,
					fmt.Sprintf("mountPath %q is nested under %q (index %d)", path2, path1, i)))
			} else if strings.HasPrefix(path1+"/", path2+"/") {
				// path1 is nested under path2
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "volumeMounts").Index(i).Child("mountPath"),
					path1,
					fmt.Sprintf("mountPath %q is nested under %q (index %d)", path1, path2, j)))
			}
		}
	}

	return errs
}

// validateVolumeSource validates volume source configurations.
//
// This webhook implements only operator-specific security policies:
//   - hostPath volume blocking (security policy)
//
// The following validations are intentionally NOT implemented here because Kubernetes API server
// already validates them when creating the underlying Deployment:
//   - Empty ConfigMap/Secret/PVC names (K8s required field validation)
//   - Volume source configuration format (K8s schema validation)
func (v *AgentCustomValidator) validateVolumeSource(volume corev1.Volume, index int) []*field.Error {
	var errs []*field.Error
	basePath := field.NewPath("spec", "volumes").Index(index)

	// Block hostPath unless explicitly allowed (operator-specific security policy)
	if volume.HostPath != nil && !v.AllowHostPath {
		errs = append(errs, field.Forbidden(
			basePath.Child("hostPath"),
			"hostPath volumes are not allowed for security reasons. "+
				"Contact your cluster administrator if you need this feature."))
	}

	return errs
}
