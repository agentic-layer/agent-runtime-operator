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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// log is for logging in this package.
var agenticworkforcelog = logf.Log.WithName("agenticworkforce-resource")

// SetupAgenticWorkforceWebhookWithManager registers the webhook for AgenticWorkforce in the manager.
func SetupAgenticWorkforceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.AgenticWorkforce{}).
		WithValidator(&AgenticWorkforceCustomValidator{
			Client: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-runtime-agentic-layer-ai-v1alpha1-agenticworkforce,mutating=false,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agenticworkforces,verbs=create;update,versions=v1alpha1,name=vagenticworkforce-v1alpha1.kb.io,admissionReviewVersions=v1

// AgenticWorkforceCustomValidator struct is responsible for validating the AgenticWorkforce resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AgenticWorkforceCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &AgenticWorkforceCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the Kind AgenticWorkforce.
func (v *AgenticWorkforceCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	workforce, ok := obj.(*runtimev1alpha1.AgenticWorkforce)
	if !ok {
		return nil, fmt.Errorf("expected an AgenticWorkforce object but got %T", obj)
	}
	agenticworkforcelog.Info("Validating AgenticWorkforce on create", "name", workforce.GetName())
	return v.validateAgenticWorkforce(ctx, workforce)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the Kind AgenticWorkforce.
func (v *AgenticWorkforceCustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	workforce, ok := newObj.(*runtimev1alpha1.AgenticWorkforce)
	if !ok {
		return nil, fmt.Errorf("expected an AgenticWorkforce object but got %T", newObj)
	}
	agenticworkforcelog.Info("Validating AgenticWorkforce on update", "name", workforce.GetName())
	return v.validateAgenticWorkforce(ctx, workforce)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the Kind AgenticWorkforce.
func (v *AgenticWorkforceCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// No validation needed on delete
	return nil, nil
}

// validateAgenticWorkforce performs validation logic for AgenticWorkforce resources.
// Note: Simple required field validation (name, description, owner, entryPointAgents) is handled
// via kubebuilder markers in the CRD. This webhook focuses on complex validation that requires
// runtime checks, such as verifying that referenced agents exist.
func (v *AgenticWorkforceCustomValidator) validateAgenticWorkforce(ctx context.Context, workforce *runtimev1alpha1.AgenticWorkforce) (admission.Warnings, error) {
	var allErrs field.ErrorList

	// Validate each entry point agent reference exists in the cluster
	entryPointsPath := field.NewPath("spec").Child("entryPointAgents")
	for i, agentRef := range workforce.Spec.EntryPointAgents {
		agentPath := entryPointsPath.Index(i)

		// Skip nil references (should be caught by CRD validation, but check defensively)
		if agentRef == nil {
			continue
		}

		// Skip empty names (should be caught by CRD validation, but check defensively)
		if agentRef.Name == "" {
			continue
		}

		// Determine the namespace to look in
		namespace := agentRef.Namespace
		if namespace == "" {
			namespace = workforce.Namespace
		}

		// Check if the referenced agent exists
		agent := &runtimev1alpha1.Agent{}
		err := v.Client.Get(ctx, types.NamespacedName{
			Name:      agentRef.Name,
			Namespace: namespace,
		}, agent)

		if err != nil {
			allErrs = append(allErrs, field.Invalid(
				agentPath,
				agentRef.Name,
				fmt.Sprintf("agent '%s' not found in namespace '%s': %v", agentRef.Name, namespace, err),
			))
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}
