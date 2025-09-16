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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	googleAdkFramework = "google-adk"
)

// nolint:unused
// log is for logging in this package.
var agentlog = logf.Log.WithName("agent-resource")

// SetupAgentWebhookWithManager registers the webhook for Agent in the manager.
func SetupAgentWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&runtimev1alpha1.Agent{}).
		WithDefaulter(&AgentCustomDefaulter{
			DefaultReplicas:      1,
			DefaultPort:          8080,
			DefaultPortGoogleAdk: 8000,
			Recorder:             mgr.GetEventRecorderFor("agent-defaulter-webhook"),
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
	// Set default replicas if not specified
	if agent.Spec.Replicas == nil {
		agent.Spec.Replicas = new(int32)
		*agent.Spec.Replicas = d.DefaultReplicas
	}

	// Set default ports for protocols if not specified
	for i, protocol := range agent.Spec.Protocols {
		if protocol.Port == 0 {
			agent.Spec.Protocols[i].Port = d.frameworkDefaultPort(agent.Spec.Framework)
		}
		if protocol.Name == "" {
			agent.Spec.Protocols[i].Name = fmt.Sprintf("%s-%d", protocol.Type, protocol.Port)
		}
	}

	// Filter out protected environment variables and create an event if found.
	protectedVar := "AGENT_NAME"
	var filteredEnvs []corev1.EnvVar

	for _, env := range agent.Spec.Env {
		if env.Name != protectedVar {
			filteredEnvs = append(filteredEnvs, env)
		} else {
			agentlog.Info("removing protected environment variable from spec", "variable", protectedVar, "agent", agent.GetName())
			d.Recorder.Eventf(agent, "Warning", "SpecModified", "The user-defined '%s' environment variable was removed as it is system-managed.", protectedVar)
		}
	}
	agent.Spec.Env = filteredEnvs
}

func (d *AgentCustomDefaulter) frameworkDefaultPort(framework string) int32 {
	switch framework {
	case googleAdkFramework:
		return d.DefaultPortGoogleAdk
	default:
		return d.DefaultPort // Default port for unknown frameworks
	}
}
