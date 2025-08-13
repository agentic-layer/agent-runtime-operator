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
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var agentlog = logf.Log.WithName("agent-resource")

// SetupAgentWebhookWithManager registers the webhook for Agent in the manager.
func SetupAgentWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.Agent{}).
		WithDefaulter(&AgentCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-runtime-agentic-layer-ai-v1alpha1-agent,mutating=true,failurePolicy=fail,sideEffects=None,groups=runtime.agentic-layer.ai,resources=agents,verbs=create;update,versions=v1alpha1,name=magent-v1alpha1.kb.io,admissionReviewVersions=v1

// AgentCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Agent when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AgentCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &AgentCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Agent.
func (d *AgentCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	agent, ok := obj.(*runtimev1alpha1.Agent)

	if !ok {
		return fmt.Errorf("expected an Agent object but got %T", obj)
	}
	agentlog.Info("Defaulting for Agent", "name", agent.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}
