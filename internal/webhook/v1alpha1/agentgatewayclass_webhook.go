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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	DefaultClassAnnotation = "agentgatewayclass.kubernetes.io/is-default-class"
)

// log is for logging in this package.
var agentgatewayclasslog = logf.Log.WithName("agentgatewayclass-resource")

// SetupAgentGatewayClassWebhookWithManager registers the webhook for AgentGatewayClass in the manager.
func SetupAgentGatewayClassWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.AgentGatewayClass{}).
		WithValidator(&AgentGatewayClassCustomValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-runtime-agentic-layer-ai-v1alpha1-agentgatewayclass,mutating=false,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agentgatewayclasses,verbs=create;update,versions=v1alpha1,name=vagentgatewayclass-v1alpha1.kb.io,admissionReviewVersions=v1

// +kubebuilder:rbac:groups=runtime.agentic-layer.ai,resources=agentgatewayclasses,verbs=get;list;watch

// AgentGatewayClassCustomValidator struct is responsible for validating the AgentGatewayClass resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AgentGatewayClassCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &AgentGatewayClassCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the Kind AgentGatewayClass.
func (v *AgentGatewayClassCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	agentGatewayClass, ok := obj.(*runtimev1alpha1.AgentGatewayClass)
	if !ok {
		return nil, fmt.Errorf("expected an AgentGatewayClass object but got %T", obj)
	}
	agentgatewayclasslog.Info("Validating AgentGatewayClass on create", "name", agentGatewayClass.GetName())
	return v.validateAgentGatewayClass(ctx, agentGatewayClass)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the Kind AgentGatewayClass.
func (v *AgentGatewayClassCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	agentGatewayClass, ok := newObj.(*runtimev1alpha1.AgentGatewayClass)
	if !ok {
		return nil, fmt.Errorf("expected an AgentGatewayClass object but got %T", newObj)
	}
	agentgatewayclasslog.Info("Validating AgentGatewayClass on update", "name", agentGatewayClass.GetName())
	return v.validateAgentGatewayClass(ctx, agentGatewayClass)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the Kind AgentGatewayClass.
func (v *AgentGatewayClassCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// No validation needed on delete
	return nil, nil
}

// validateAgentGatewayClass performs validation logic for AgentGatewayClass resources.
func (v *AgentGatewayClassCustomValidator) validateAgentGatewayClass(ctx context.Context, agentGatewayClass *runtimev1alpha1.AgentGatewayClass) (admission.Warnings, error) {
	var allErrs field.ErrorList

	// Check if this AgentGatewayClass has the default class annotation set to "true"
	annotations := agentGatewayClass.GetAnnotations()
	if annotations != nil && annotations[DefaultClassAnnotation] == "true" {
		// List all existing AgentGatewayClass resources
		var agentGatewayClassList runtimev1alpha1.AgentGatewayClassList
		if err := v.Client.List(ctx, &agentGatewayClassList); err != nil {
			return nil, fmt.Errorf("failed to list AgentGatewayClass resources: %w", err)
		}

		// Check if any other AgentGatewayClass already has the default annotation
		for _, existingClass := range agentGatewayClassList.Items {
			// Skip the current resource being validated
			if existingClass.GetName() == agentGatewayClass.GetName() {
				continue
			}

			existingAnnotations := existingClass.GetAnnotations()
			if existingAnnotations != nil && existingAnnotations[DefaultClassAnnotation] == "true" {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("metadata", "annotations").Key(DefaultClassAnnotation),
					"true",
					fmt.Sprintf("another AgentGatewayClass '%s' already has the default class annotation set to 'true'. Only one AgentGatewayClass can be marked as default", existingClass.GetName()),
				))
				break
			}
		}
	}

	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	return nil, nil
}
