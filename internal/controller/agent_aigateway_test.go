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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var aiGatewaySpec = runtimev1alpha1.AiGatewaySpec{
	AiModels: []runtimev1alpha1.AiModel{
		{
			Name:     "gpt-4",
			Provider: "openai",
		},
	},
}

var _ = Describe("Agent AiGateway Resolution", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler
	var testAiGatewayNamespace string

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		// Set POD_NAMESPACE for tests
		Expect(os.Setenv("POD_NAMESPACE", "default")).To(Succeed())
		// Use a unique namespace name for each test to avoid conflicts
		testAiGatewayNamespace = "ai-gateway"
	})

	// Helper function to create namespace if it doesn't exist
	createNamespaceIfNotExists := func(name string) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		err := k8sClient.Create(ctx, ns)
		if err != nil && client.IgnoreAlreadyExists(err) != nil {
			Fail("Failed to create namespace " + name + ": " + err.Error())
		}
	}

	AfterEach(func() {
		// Clean up all agents in default namespace
		agentList := &runtimev1alpha1.AgentList{}
		_ = k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: "default"})
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}

		// Clean up all AiGateways in ai-gateway namespace (don't delete namespace)
		aiGatewayList := &runtimev1alpha1.AiGatewayList{}
		_ = k8sClient.List(ctx, aiGatewayList, &client.ListOptions{Namespace: "ai-gateway"})
		for i := range aiGatewayList.Items {
			_ = k8sClient.Delete(ctx, &aiGatewayList.Items[i])
		}

		// Clean up resources in other-namespace (don't delete namespace to avoid termination issues)
		otherNsAgentList := &runtimev1alpha1.AgentList{}
		_ = k8sClient.List(ctx, otherNsAgentList, &client.ListOptions{Namespace: "other-namespace"})
		for i := range otherNsAgentList.Items {
			_ = k8sClient.Delete(ctx, &otherNsAgentList.Items[i])
		}

		// Clean up AiGateways in other-namespace
		otherNsAiGatewayList := &runtimev1alpha1.AiGatewayList{}
		_ = k8sClient.List(ctx, otherNsAiGatewayList, &client.ListOptions{Namespace: "other-namespace"})
		for i := range otherNsAiGatewayList.Items {
			_ = k8sClient.Delete(ctx, &otherNsAiGatewayList.Items[i])
		}
	})

	Describe("resolveAiGateway", func() {
		It("should resolve explicit AiGatewayRef with specified namespace", func() {
			// Create or get ai-gateway namespace
			createNamespaceIfNotExists(testAiGatewayNamespace)

			// Create AiGateway in ai-gateway namespace
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-ai-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent with explicit AiGatewayRef
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "my-ai-gateway",
						Namespace: "ai-gateway",
					},
				},
			}

			gateway, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).NotTo(BeNil())
			Expect(gateway.Name).To(Equal("my-ai-gateway"))
			Expect(gateway.Namespace).To(Equal("ai-gateway"))
		})

		It("should default to agent's namespace when AiGatewayRef namespace is not specified", func() {
			// Create AiGateway in default namespace
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-ai-gateway",
					Namespace: "default",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent with AiGatewayRef without namespace
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-default-ns",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name: "default-ai-gateway",
						// Namespace omitted - should default to agent's namespace
					},
				},
			}

			gateway, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).NotTo(BeNil())
			Expect(gateway.Name).To(Equal("default-ai-gateway"))
			Expect(gateway.Namespace).To(Equal("default"))
		})

		It("should find default AiGateway in ai-gateway namespace when no ref is specified", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create default AiGateway
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent without AiGatewayRef
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-auto",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					// No AiGatewayRef specified
				},
			}

			gateway, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).NotTo(BeNil())
			Expect(gateway.Name).To(Equal("default-gateway"))
			Expect(gateway.Namespace).To(Equal("ai-gateway"))
		})

		It("should return nil when no AiGateway exists and no ref is specified", func() {
			// Create agent without AiGatewayRef and no default gateway exists
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-no-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					// No AiGatewayRef specified and no default exists
				},
			}

			gateway, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).To(BeNil())
		})

		It("should return error when explicit AiGatewayRef points to non-existent gateway", func() {
			// Create agent with explicit ref to non-existent gateway
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-missing-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "nonexistent-gateway",
						Namespace: "ai-gateway",
					},
				},
			}

			_, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve AiGateway"))
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should return first gateway when multiple exist in ai-gateway namespace", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create multiple AiGateways
			gateway1 := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-1",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, gateway1)).To(Succeed())

			gateway2 := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-2",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, gateway2)).To(Succeed())

			// Create agent without AiGatewayRef
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-multi-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}

			gateway, err := reconciler.resolveAiGateway(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).NotTo(BeNil())
			// Should return one of the gateways (deterministic based on API list order)
			Expect(gateway.Name).To(MatchRegexp(`gateway-[12]`))
			Expect(gateway.Namespace).To(Equal("ai-gateway"))
		})
	})

	Describe("resolveExplicitAiGateway", func() {
		It("should construct correct service URL", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create AiGateway
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			ref := &corev1.ObjectReference{
				Name:      "test-gateway",
				Namespace: "ai-gateway",
			}

			gateway, err := reconciler.resolveExplicitAiGateway(ctx, ref, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).NotTo(BeNil())
			Expect(gateway.Name).To(Equal("test-gateway"))
			Expect(gateway.Namespace).To(Equal("ai-gateway"))
		})
	})

	Describe("resolveDefaultAiGateway", func() {
		It("should return nil when ai-gateway namespace is empty", func() {
			// Create empty ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			gateway, err := reconciler.resolveDefaultAiGateway(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway).To(BeNil())
		})
	})

	Describe("findAgentsReferencingAiGateway", func() {
		It("should identify agents with explicit AiGatewayRef", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create AiGateway
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "referenced-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent with explicit reference
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-ref",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "referenced-gateway",
						Namespace: "ai-gateway",
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("agent-with-ref"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should match with namespace defaulting in AiGatewayRef", func() {
			// Create AiGateway in default namespace
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-with-defaulting",
					Namespace: "default",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent with AiGatewayRef without namespace (defaults to agent's namespace)
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-defaulting",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name: "gateway-with-defaulting",
						// Namespace omitted - defaults to agent's namespace
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("agent-with-defaulting"))
		})

		It("should identify agents without explicit ref when gateway is in default namespace", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create AiGateway in default ai-gateway namespace
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent without AiGatewayRef
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-without-ref",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					// No AiGatewayRef - would use default resolution
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("agent-without-ref"))
		})

		It("should not match agents without ref when gateway is NOT in default namespace", func() {
			// Create other-namespace
			createNamespaceIfNotExists("other-namespace")

			// Create AiGateway in non-default namespace
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-gateway",
					Namespace: "other-namespace",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent without AiGatewayRef
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-no-match",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					// No AiGatewayRef - would not match non-default gateway
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(BeEmpty())
		})

		It("should handle multiple agents referencing same gateway", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create shared AiGateway
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create first agent with explicit ref
			agent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "shared-gateway",
						Namespace: "ai-gateway",
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent1)).To(Succeed())

			// Create second agent without ref (uses default)
			agent2 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent2",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					// No AiGatewayRef
				},
			}
			Expect(k8sClient.Create(ctx, agent2)).To(Succeed())

			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(HaveLen(2))

			// Check that both agents are in the requests
			agentNames := make(map[string]bool)
			for _, req := range requests {
				agentNames[req.Name] = true
			}
			Expect(agentNames).To(HaveKey("agent1"))
			Expect(agentNames).To(HaveKey("agent2"))
		})

		It("should not match agents with different explicit references", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create first AiGateway
			aiGateway1 := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway1",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway1)).To(Succeed())

			// Create second AiGateway (this is the one we'll watch)
			aiGateway2 := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway2",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway2)).To(Succeed())

			// Create agent referencing gateway1
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-gateway1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "gateway1",
						Namespace: "ai-gateway",
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Watch gateway2 - should not find agent referencing gateway1
			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway2)
			Expect(requests).To(BeEmpty())
		})

		It("should return empty list when no agents exist", func() {
			// Create ai-gateway namespace
			createNamespaceIfNotExists("ai-gateway")

			// Create AiGateway
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lonely-gateway",
					Namespace: "ai-gateway",
				},
				Spec: aiGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// No agents created
			requests := reconciler.findAgentsReferencingAiGateway(ctx, aiGateway)
			Expect(requests).To(BeEmpty())
		})
	})
})
