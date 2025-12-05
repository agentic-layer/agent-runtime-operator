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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("AgenticWorkforce Controller", func() {
	Context("Reconciliation Flow", func() {
		const workforceName = "test-workforce"
		const agentName = "test-agent"

		ctx := context.Background()

		BeforeEach(func() {
			// Create test agent
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: "A2A", Port: 8000},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
		})

		AfterEach(func() {
			// Clean up agent
			agent := &runtimev1alpha1.Agent{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: agentName, Namespace: "default"}, agent); err == nil {
				Expect(k8sClient.Delete(ctx, agent)).To(Succeed())
			}

			// Clean up workforce
			workforce := &runtimev1alpha1.AgenticWorkforce{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: workforceName, Namespace: "default"}, workforce); err == nil {
				Expect(k8sClient.Delete(ctx, workforce)).To(Succeed())
			}
		})

		It("should successfully reconcile workforce with valid agents", func() {
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workforceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: agentName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workforceName,
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status was updated with Ready condition
			updatedWorkforce := &runtimev1alpha1.AgenticWorkforce{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: workforceName, Namespace: "default"}, updatedWorkforce)).To(Succeed())
			Expect(updatedWorkforce.Status.Conditions).To(HaveLen(1))
			Expect(updatedWorkforce.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(updatedWorkforce.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(updatedWorkforce.Status.Conditions[0].Reason).To(Equal("AllAgentsReady"))
		})

		It("should handle AgenticWorkforce not found without error", func() {
			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent-workforce",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update status correctly when agents are missing", func() {
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workforceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Test Workforce",
					Description: "A test workforce",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "missing-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workforceName,
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status shows Not Ready condition
			updatedWorkforce := &runtimev1alpha1.AgenticWorkforce{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: workforceName, Namespace: "default"}, updatedWorkforce)).To(Succeed())
			Expect(updatedWorkforce.Status.Conditions).To(HaveLen(1))
			Expect(updatedWorkforce.Status.Conditions[0].Type).To(Equal("Ready"))
			Expect(updatedWorkforce.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(updatedWorkforce.Status.Conditions[0].Reason).To(Equal("AgentsMissing"))
			Expect(updatedWorkforce.Status.Conditions[0].Message).To(ContainSubstring("default/missing-agent"))
		})
	})

	Context("Entry Point Agent Validation", func() {
		var reconciler *AgenticWorkforceReconciler
		ctx := context.Background()

		BeforeEach(func() {
			reconciler = &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should return true when all agents exist in same namespace", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "valid-agent"},
					},
				},
			}

			allReady, missingAgents := reconciler.validateEntryPointAgents(ctx, workforce)
			Expect(allReady).To(BeTrue())
			Expect(missingAgents).To(BeEmpty())
		})

		It("should return false when agent is missing", func() {
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "missing-agent"},
					},
				},
			}

			allReady, missingAgents := reconciler.validateEntryPointAgents(ctx, workforce)
			Expect(allReady).To(BeFalse())
			Expect(missingAgents).To(HaveLen(1))
			Expect(missingAgents[0]).To(Equal("default/missing-agent"))
		})

		It("should default namespace to workforce namespace when not specified", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-no-ns",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-no-ns"}, // No namespace specified
					},
				},
			}

			allReady, missingAgents := reconciler.validateEntryPointAgents(ctx, workforce)
			Expect(allReady).To(BeTrue())
			Expect(missingAgents).To(BeEmpty())
		})

		It("should find agent in different namespace when specified", func() {
			// Use a unique namespace name to avoid conflicts with terminating namespaces
			testNamespace := "test-ns-" + string(types.UID(metav1.Now().Format("20060102-150405")))

			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, ns) }()

			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-agent",
					Namespace: testNamespace,
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "cross-ns-agent", Namespace: testNamespace},
					},
				},
			}

			allReady, missingAgents := reconciler.validateEntryPointAgents(ctx, workforce)
			Expect(allReady).To(BeTrue())
			Expect(missingAgents).To(BeEmpty())
		})
	})

	Context("Transitive Collection", func() {
		var reconciler *AgenticWorkforceReconciler
		ctx := context.Background()

		BeforeEach(func() {
			reconciler = &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should collect tools from agent both ToolServer references and direct URL tools", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-mixed-tools",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "local-tool", ToolServerRef: &corev1.ObjectReference{Name: "local-server"}},
						{Name: "remote-tool", Url: "https://mcp.example.com/tools"},
						{Name: "another-local", ToolServerRef: &corev1.ObjectReference{Name: "another-server", Namespace: "tools"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-with-mixed-tools"},
					},
				},
			}

			agents, tools := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(ContainElement(runtimev1alpha1.TransitiveAgent{
				Name:      "agent-with-mixed-tools",
				Namespace: "default",
			}))
			Expect(tools).To(HaveLen(3))
			Expect(tools).To(ContainElements(
				"default/local-server",          // ToolServerRef format
				"tools/another-server",          // ToolServerRef with namespace
				"https://mcp.example.com/tools", // Direct URL format
			))
		})

		It("should collect cluster sub-agent using AgentRef", func() {
			subAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sub-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, subAgent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, subAgent) }()

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name: "sub",
							AgentRef: &corev1.ObjectReference{
								Name:      "sub-agent",
								Namespace: "default",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, parentAgent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "parent-agent"},
					},
				},
			}

			agents, _ := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(HaveLen(2))
			Expect(agents).To(ContainElements(
				runtimev1alpha1.TransitiveAgent{Name: "parent-agent", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "sub-agent", Namespace: "default"},
			))
		})

		It("should collect remote sub-agent using Url", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-remote",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name: "remote-sub",
							Url:  "https://remote-agent.example.com/.well-known/agent-card.json",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-with-remote"},
					},
				},
			}

			agents, _ := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(HaveLen(2))
			Expect(agents).To(ContainElements(
				runtimev1alpha1.TransitiveAgent{Name: "agent-with-remote", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "remote-sub", Url: "https://remote-agent.example.com/.well-known/agent-card.json"},
			))
		})

		It("should handle multi-level hierarchy", func() {
			agentC := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-c",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "tool-c", ToolServerRef: &corev1.ObjectReference{Name: "tool-c-server"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentC)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agentC) }()

			agentB := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-b",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "sub-c",
							AgentRef: &corev1.ObjectReference{Name: "agent-c"},
						},
					},
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "tool-b", ToolServerRef: &corev1.ObjectReference{Name: "tool-b-server"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentB)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agentB) }()

			agentA := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-a",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "sub-b",
							AgentRef: &corev1.ObjectReference{Name: "agent-b"},
						},
					},
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "tool-a", ToolServerRef: &corev1.ObjectReference{Name: "tool-a-server"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentA)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agentA) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-a"},
					},
				},
			}

			agents, tools := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(HaveLen(3))
			Expect(agents).To(ContainElements(
				runtimev1alpha1.TransitiveAgent{Name: "agent-a", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "agent-b", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "agent-c", Namespace: "default"},
			))
			Expect(tools).To(HaveLen(3))
		})

		It("should handle circular references without infinite loop", func() {
			agentB := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-circular-b",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "back-to-a",
							AgentRef: &corev1.ObjectReference{Name: "agent-circular-a"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentB)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agentB) }()

			agentA := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-circular-a",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "to-b",
							AgentRef: &corev1.ObjectReference{Name: "agent-circular-b"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentA)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agentA) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-circular-a"},
					},
				},
			}

			agents, _ := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(HaveLen(2))
			Expect(agents).To(ContainElements(
				runtimev1alpha1.TransitiveAgent{Name: "agent-circular-a", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "agent-circular-b", Namespace: "default"},
			))
		})

		It("should continue when agent is missing during traversal", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-missing-sub",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "missing-sub",
							AgentRef: &corev1.ObjectReference{Name: "nonexistent-agent"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "agent-with-missing-sub"},
					},
				},
			}

			agents, _ := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(ContainElement(runtimev1alpha1.TransitiveAgent{
				Name:      "agent-with-missing-sub",
				Namespace: "default",
			}))
		})

		It("should deduplicate agents from multiple entry points", func() {
			sharedAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, sharedAgent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, sharedAgent) }()

			entryAgent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "entry-1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "shared",
							AgentRef: &corev1.ObjectReference{Name: "shared-agent"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, entryAgent1)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, entryAgent1) }()

			entryAgent2 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "entry-2",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name:     "shared",
							AgentRef: &corev1.ObjectReference{Name: "shared-agent"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, entryAgent2)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, entryAgent2) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "entry-1"},
						{Name: "entry-2"},
					},
				},
			}

			agents, _ := reconciler.collectTransitiveAgentsAndTools(ctx, workforce)
			Expect(agents).To(HaveLen(3))
			Expect(agents).To(ContainElements(
				runtimev1alpha1.TransitiveAgent{Name: "entry-1", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "entry-2", Namespace: "default"},
				runtimev1alpha1.TransitiveAgent{Name: "shared-agent", Namespace: "default"},
			))
		})
	})

	Context("Agent Watch Mapper", func() {
		ctx := context.Background()

		It("should return empty list when agent doesn't match any workforces", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-agent",
					Namespace: "default",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(BeEmpty())
		})

		It("should return workforce when agent is directly referenced as entry point", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "direct-ref-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Watch Test Workforce",
					Description: "Test workforce for watch mapper",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "direct-ref-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce) }()

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("watch-workforce-1"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should return workforce when agent appears in transitive agents", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "transitive-ref-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			// Create another agent to use as entry point
			entryAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "transitive-entry-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, entryAgent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, entryAgent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-2",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Watch Test Workforce 2",
					Description: "Test workforce with transitive reference",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "transitive-entry-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce) }()

			// Update status separately (status is a subresource)
			workforce.Status.TransitiveAgents = []runtimev1alpha1.TransitiveAgent{
				{Name: "transitive-entry-agent", Namespace: "default"},
				{Name: "transitive-ref-agent", Namespace: "default"},
			}
			Expect(k8sClient.Status().Update(ctx, workforce)).To(Succeed())

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("watch-workforce-2"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should handle namespace defaulting correctly", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ns-default-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			// Workforce references agent without specifying namespace (should default to workforce namespace)
			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-3",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Watch Test Workforce 3",
					Description: "Test workforce with namespace defaulting",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "ns-default-agent"}, // No namespace specified
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce) }()

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("watch-workforce-3"))
		})

		It("should return multiple workforces if agent is referenced by multiple workforces", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-ref-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce1 := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-4a",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Watch Test Workforce 4a",
					Description: "First workforce referencing agent",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "multi-ref-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce1)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce1) }()

			workforce2 := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-4b",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Watch Test Workforce 4b",
					Description: "Second workforce referencing agent",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "multi-ref-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce2)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce2) }()

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(HaveLen(2))

			workforceNames := []string{requests[0].Name, requests[1].Name}
			Expect(workforceNames).To(ContainElements("watch-workforce-4a", "watch-workforce-4b"))
		})

		It("should return empty list for invalid input", func() {
			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Pass a non-Agent object
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
				},
			}

			requests := reconciler.findWorkforcesReferencingAgent(ctx, ns)
			Expect(requests).To(BeEmpty())
		})

		It("should deduplicate workforce when agent appears in both entry points and transitive agents", func() {
			// Entry point agents always appear in TransitiveAgents, so without deduplication, the workforce would be
			// enqueued twice when an entry point agent changes.

			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dedup-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, agent) }()

			workforce := &runtimev1alpha1.AgenticWorkforce{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "watch-workforce-dedup",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgenticWorkforceSpec{
					Name:        "Deduplication Test Workforce",
					Description: "Test workforce for deduplication",
					Owner:       "test@example.com",
					EntryPointAgents: []*corev1.ObjectReference{
						{Name: "dedup-agent"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workforce)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, workforce) }()

			// Update status to include the same agent in transitive agents
			// (this is the typical scenario after reconciliation)
			workforce.Status.TransitiveAgents = []runtimev1alpha1.TransitiveAgent{
				{Name: "dedup-agent", Namespace: "default"},
			}
			Expect(k8sClient.Status().Update(ctx, workforce)).To(Succeed())

			reconciler := &AgenticWorkforceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// The agent appears in BOTH EntryPointAgents and TransitiveAgents
			// Without deduplication, this would return 2 requests
			// With deduplication, it should return exactly 1 request
			requests := reconciler.findWorkforcesReferencingAgent(ctx, agent)
			Expect(requests).To(HaveLen(1), "Workforce should be enqueued exactly once, not twice")
			Expect(requests[0].Name).To(Equal("watch-workforce-dedup"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})
	})
})
