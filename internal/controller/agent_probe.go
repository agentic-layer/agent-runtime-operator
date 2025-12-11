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

package controller

import (
	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// findA2AProtocol returns the first A2A protocol configuration found
func findA2AProtocol(agent *runtimev1alpha1.Agent) *runtimev1alpha1.AgentProtocol {
	for _, protocol := range agent.Spec.Protocols {
		if protocol.Type == runtimev1alpha1.A2AProtocol {
			return &protocol
		}
	}
	return nil
}

// generateReadinessProbe generates appropriate readiness probe based on agent protocols
func generateReadinessProbe(agent *runtimev1alpha1.Agent) *corev1.Probe {
	// Use A2A agent card endpoint for health check
	protocol := findA2AProtocol(agent)
	if protocol != nil {
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: protocol.Path + agentCardEndpoint,
					Port: intstr.FromInt32(protocol.Port),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      3,
			SuccessThreshold:    1,
			FailureThreshold:    10,
		}
	}
	return nil
}
