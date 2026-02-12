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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var toolgatewaylog = logf.Log.WithName("toolgateway-resource")

// SetupToolGatewayWebhookWithManager registers the webhook for ToolGateway in the manager.
func SetupToolGatewayWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.ToolGateway{}).
		WithDefaulter(&ToolGatewayCustomDefaulter{
			Recorder: mgr.GetEventRecorderFor("toolgateway-defaulter-webhook"),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-runtime-agentic-layer-ai-v1alpha1-toolgateway,mutating=true,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=toolgateways,verbs=create;update,versions=v1alpha1,name=mtoolgateway-v1alpha1.kb.io,admissionReviewVersions=v1

// ToolGatewayCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ToolGateway when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ToolGatewayCustomDefaulter struct {
	Recorder record.EventRecorder
}

var _ webhook.CustomDefaulter = &ToolGatewayCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ToolGateway.
func (d *ToolGatewayCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	toolgateway, ok := obj.(*runtimev1alpha1.ToolGateway)

	if !ok {
		return fmt.Errorf("expected a ToolGateway object but got %T", obj)
	}
	toolgatewaylog.Info("Defaulting for ToolGateway", "name", toolgateway.GetName())

	// No defaults to apply currently
	return nil
}
