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
	googleAdkFramework           = "google-adk"
	DefaultTemplateImageAdk      = "ghcr.io/agentic-layer/agent-template-adk:0.3.1"
	defaultTemplateImageFallback = "invalid"
	operatorVolumePrefix         = "agent-operator-"
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

	// Validate Tool URLs (only scheme validation, kubebuilder handles URI format)
	for i, tool := range agent.Spec.Tools {
		if tool.Url != "" {
			if err := v.validateHTTPScheme(tool.Url); err != nil {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "tools").Index(i).Child("url"),
					tool.Url,
					fmt.Sprintf("Tool[%d].Url: %s", i, err.Error()),
				))
			}
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

// validateVolumeMounts validates that all volumeMounts reference volumes that exist in the volumes list.
func (v *AgentCustomValidator) validateVolumeMounts(agent *runtimev1alpha1.Agent) []*field.Error {
	var errs []*field.Error

	// Build a set of available volume names and check for duplicates
	volumeNames := make(map[string]bool)
	for i, volume := range agent.Spec.Volumes {
		if volume.Name == "" {
			errs = append(errs, field.Required(
				field.NewPath("spec", "volumes").Index(i).Child("name"),
				"volume must have a name"))
		} else if strings.HasPrefix(volume.Name, operatorVolumePrefix) {
			// Reject reserved volume names
			errs = append(errs, field.Forbidden(
				field.NewPath("spec", "volumes").Index(i).Child("name"),
				fmt.Sprintf("volume names starting with %q are reserved for operator use", operatorVolumePrefix)))
		} else if volumeNames[volume.Name] {
			// Check for duplicate volume names
			errs = append(errs, field.Duplicate(
				field.NewPath("spec", "volumes").Index(i).Child("name"),
				volume.Name))
			// Already in map, don't re-add
		} else {
			// Valid, unique volume name - add to tracking map
			volumeNames[volume.Name] = true
		}
	}

	// Track seen mount paths to detect duplicates
	seenMountPaths := make(map[string]bool)

	// Check each volumeMount references an existing volume
	for i, volumeMount := range agent.Spec.VolumeMounts {
		if volumeMount.Name == "" {
			errs = append(errs, field.Required(
				field.NewPath("spec", "volumeMounts").Index(i).Child("name"),
				"volumeMount must have a name"))
			continue
		}

		if !volumeNames[volumeMount.Name] {
			errs = append(errs, field.Invalid(
				field.NewPath("spec", "volumeMounts").Index(i).Child("name"),
				volumeMount.Name,
				fmt.Sprintf("volumeMount references volume %q which does not exist in spec.volumes", volumeMount.Name)))
		}

		// Validate mountPath
		if volumeMount.MountPath == "" {
			errs = append(errs, field.Required(
				field.NewPath("spec", "volumeMounts").Index(i).Child("mountPath"),
				"mountPath is required"))
			continue
		}

		// Check mountPath does not contain ':' (Kubernetes requirement)
		if strings.Contains(volumeMount.MountPath, ":") {
			errs = append(errs, field.Invalid(
				field.NewPath("spec", "volumeMounts").Index(i).Child("mountPath"),
				volumeMount.MountPath,
				"mountPath must not contain ':'"))
		}

		// Check for duplicate mount paths
		if seenMountPaths[volumeMount.MountPath] {
			errs = append(errs, field.Duplicate(
				field.NewPath("spec", "volumeMounts").Index(i).Child("mountPath"),
				volumeMount.MountPath))
		}
		seenMountPaths[volumeMount.MountPath] = true

		// Check for overlapping/nested mount paths
		for j := i + 1; j < len(agent.Spec.VolumeMounts); j++ {
			if agent.Spec.VolumeMounts[j].MountPath == "" {
				continue // Skip if mountPath is empty (will be caught by validation above)
			}

			path1 := filepath.Clean(volumeMount.MountPath)
			path2 := filepath.Clean(agent.Spec.VolumeMounts[j].MountPath)

			// Check if one path is a parent of the other
			// We need to add trailing slash to prevent false positives like /data and /data-backup
			if strings.HasPrefix(path2+"/", path1+"/") {
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "volumeMounts").Index(j).Child("mountPath"),
					path2,
					fmt.Sprintf("mountPath %q overlaps with volumeMount at index %d (%q). Kubernetes does not allow nested mount paths", path2, i, path1)))
			} else if strings.HasPrefix(path1+"/", path2+"/") {
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "volumeMounts").Index(i).Child("mountPath"),
					path1,
					fmt.Sprintf("mountPath %q overlaps with volumeMount at index %d (%q). Kubernetes does not allow nested mount paths", path1, j, path2)))
			}
		}
	}

	return errs
}

// validateVolumeSource validates volume source configurations.
func (v *AgentCustomValidator) validateVolumeSource(volume corev1.Volume, index int) []*field.Error {
	var errs []*field.Error
	basePath := field.NewPath("spec", "volumes").Index(index)

	// Block hostPath unless explicitly allowed
	if volume.HostPath != nil && !v.AllowHostPath {
		errs = append(errs, field.Forbidden(
			basePath.Child("hostPath"),
			"hostPath volumes are not allowed for security reasons. "+
				"Contact your cluster administrator if you need this feature."))
	}

	// Validate ConfigMap
	if volume.ConfigMap != nil && volume.ConfigMap.Name == "" {
		errs = append(errs, field.Required(
			basePath.Child("configMap", "name"),
			"configMap name must be specified"))
	}

	// Validate Secret
	if volume.Secret != nil && volume.Secret.SecretName == "" {
		errs = append(errs, field.Required(
			basePath.Child("secret", "secretName"),
			"secret name must be specified"))
	}

	// Validate PersistentVolumeClaim
	if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == "" {
		errs = append(errs, field.Required(
			basePath.Child("persistentVolumeClaim", "claimName"),
			"persistentVolumeClaim claimName must be specified"))
	}

	return errs
}
