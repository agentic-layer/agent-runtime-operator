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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var agentgatewaylog = logf.Log.WithName("agentgateway-resource")

// SetupAgentGatewayWebhookWithManager registers the webhook for AgentGateway in the manager.
func SetupAgentGatewayWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.AgentGateway{}).
		WithDefaulter(&AgentGatewayCustomDefaulter{
			DefaultReplicas: 1,
			Recorder:        mgr.GetEventRecorderFor("agentgateway-defaulter-webhook"),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-runtime-agentic-layer-ai-v1alpha1-agentgateway,mutating=true,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agentgateways,verbs=create;update,versions=v1alpha1,name=magentgateway-v1alpha1.kb.io,admissionReviewVersions=v1

// AgentGatewayCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind AgentGateway when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AgentGatewayCustomDefaulter struct {
	DefaultReplicas int32
	Recorder        record.EventRecorder
}

var _ webhook.CustomDefaulter = &AgentGatewayCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind AgentGateway.
func (d *AgentGatewayCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	agentgateway, ok := obj.(*runtimev1alpha1.AgentGateway)

	if !ok {
		return fmt.Errorf("expected an AgentGateway object but got %T", obj)
	}
	agentgatewaylog.Info("Defaulting for AgentGateway", "name", agentgateway.GetName())

	d.applyDefaults(agentgateway)

	return nil
}

// applyDefaults applies default values to the AgentGateway.
func (d *AgentGatewayCustomDefaulter) applyDefaults(agentgateway *runtimev1alpha1.AgentGateway) {
	// Set default replicas if not specified
	if agentgateway.Spec.Replicas == nil {
		agentgateway.Spec.Replicas = new(int32)
		*agentgateway.Spec.Replicas = d.DefaultReplicas
	}

	// Set default timeout if not specified
	if agentgateway.Spec.Timeout == nil {
		agentgateway.Spec.Timeout = &metav1.Duration{Duration: 360 * time.Second}
	}
}
