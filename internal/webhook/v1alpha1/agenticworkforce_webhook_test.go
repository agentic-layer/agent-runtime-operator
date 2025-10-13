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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// NOTE: This test suite focuses on webhook-level validation (complex runtime checks).
// Simple field validation (required fields, minLength, minItems) is handled by kubebuilder
// markers in the CRD and validated by the API server before reaching the webhook.
//
// Webhook tests (this file):
// - Agent existence validation (requires Kubernetes client lookup)
// - Cross-namespace agent references
//
// CRD-level validation (not tested here):
// - Required fields: name, description, owner, entryPointAgents
// - MinLength=1: name, description, owner
// - MinItems=1: entryPointAgents array
// - Empty agent names in entryPointAgents

var _ = Describe("AgenticWorkforce Webhook", func() {
	var (
		ctx       context.Context
		validator *AgenticWorkforceCustomValidator
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		validator = &AgenticWorkforceCustomValidator{
			Client: k8sClient,
		}
		namespace = "default"

		// Clean up any existing test resources
		workforceList := &runtimev1alpha1.AgenticWorkforceList{}
		_ = k8sClient.List(ctx, workforceList, client.InNamespace(namespace))
		for i := range workforceList.Items {
			_ = k8sClient.Delete(ctx, &workforceList.Items[i])
		}

		agentList := &runtimev1alpha1.AgentList{}
		_ = k8sClient.List(ctx, agentList, client.InNamespace(namespace))
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}
	})

	Context("ValidateCreate", func() {
		It("should accept a valid workforce with existing agents", func() {
			// Create test agents
			agent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-1",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent1)).To(Succeed())

			agent2 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-2",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent2)).To(Succeed())

			// Create workforce referencing the agents
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-1"},
						{Name: "agent-2"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, workforce)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject a workforce with non-existent agent", func() {
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "non-existent-agent"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, workforce)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("agent 'non-existent-agent' not found"))
			Expect(warnings).To(BeNil())
		})

		It("should accept a workforce with cross-namespace agent reference", func() {
			otherNamespace := "other-namespace"

			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: otherNamespace,
				},
			}
			_ = k8sClient.Create(ctx, ns)

			// Create agent in other namespace
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-agent",
					Namespace: otherNamespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Create workforce in default namespace referencing agent in other namespace
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{
							Name:      "cross-ns-agent",
							Namespace: otherNamespace,
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, workforce)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())

			// Clean up
			_ = k8sClient.Delete(ctx, ns)
		})

		It("should reject a workforce with some valid and some invalid agent references", func() {
			// Create one valid agent
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-agent",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "valid-agent"},
						{Name: "invalid-agent"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, workforce)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("agent 'invalid-agent' not found"))
			Expect(warnings).To(BeNil())
		})

		It("should accept a workforce with optional tags", func() {
			// Create test agent
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-1",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					Tags:        []string{"production", "customer-support"},
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-1"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, workforce)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})

	Context("ValidateUpdate", func() {
		It("should accept valid updates to workforce", func() {
			// Create test agents
			agent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-1",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent1)).To(Succeed())

			agent2 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-2",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent2)).To(Succeed())

			// Original workforce
			oldWorkforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-1"},
					},
				},
			}

			// Updated workforce with additional agent
			newWorkforce := oldWorkforce.DeepCopy()
			newWorkforce.Spec.EntryPointAgents = []*corev1.ObjectReference{
				{Name: "agent-1"},
				{Name: "agent-2"},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldWorkforce, newWorkforce)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("should reject updates that add invalid agent references", func() {
			// Create test agent
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-1",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Original workforce
			oldWorkforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-1"},
					},
				},
			}

			// Updated workforce with invalid agent
			newWorkforce := oldWorkforce.DeepCopy()
			newWorkforce.Spec.EntryPointAgents = []*corev1.ObjectReference{
				{Name: "agent-1"},
				{Name: "non-existent-agent"},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldWorkforce, newWorkforce)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("agent 'non-existent-agent' not found"))
			Expect(warnings).To(BeNil())
		})
	})

	Context("ValidateDelete", func() {
		It("should allow deletion without validation", func() {
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workforce",
					Namespace: namespace,
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "any-agent"},
					},
				},
			}

			warnings, err := validator.ValidateDelete(ctx, workforce)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})
	})
})
