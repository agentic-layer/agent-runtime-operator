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
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent SubAgent", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		// Clean up all agents in default namespace
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}

		// Clean up agents in other-namespace (don't delete namespace to avoid termination issues)
		otherNsAgentList := &runtimev1alpha1.AgentList{}
		_ = k8sClient.List(ctx, otherNsAgentList, &client.ListOptions{Namespace: "other-namespace"})
		for i := range otherNsAgentList.Items {
			_ = k8sClient.Delete(ctx, &otherNsAgentList.Items[i])
		}
	})

	Describe("resolveSubAgentUrl", func() {
		It("should return URL directly for remote agent references", func() {
			subAgent := runtimev1alpha1.SubAgent{
				Name: "remote-agent",
				Url:  "https://example.com/.well-known/agent-card.json",
			}

			url, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://example.com/.well-known/agent-card.json"))
		})

		It("should resolve cluster agent reference with explicit namespace", func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-namespace",
				},
			}
			_ = k8sClient.Create(ctx, ns)

			clusterAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-agent",
					Namespace: "other-namespace",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, clusterAgent)).To(Succeed())

			clusterAgent.Status.Url = "http://cluster-agent.other-namespace.svc.cluster.local:8000/"
			Expect(k8sClient.Status().Update(ctx, clusterAgent)).To(Succeed())

			subAgent := runtimev1alpha1.SubAgent{
				Name: "cluster-agent",
				AgentRef: &corev1.ObjectReference{
					Name:      "cluster-agent",
					Namespace: "other-namespace",
				},
			}

			url, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://cluster-agent.other-namespace.svc.cluster.local:8000/"))
		})

		It("should default to parent agent's namespace", func() {
			clusterAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-agent-default",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, clusterAgent)).To(Succeed())

			clusterAgent.Status.Url = "http://cluster-agent-default.default.svc.cluster.local:8000/"
			Expect(k8sClient.Status().Update(ctx, clusterAgent)).To(Succeed())

			subAgent := runtimev1alpha1.SubAgent{
				Name: "cluster-agent-default",
				AgentRef: &corev1.ObjectReference{
					Name: "cluster-agent-default",
					// Namespace is omitted to test defaulting
				},
			}

			url, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://cluster-agent-default.default.svc.cluster.local:8000/"))
		})

		It("should return error when cluster agent doesn't exist", func() {
			subAgent := runtimev1alpha1.SubAgent{
				Name: "nonexistent-agent",
				AgentRef: &corev1.ObjectReference{
					Name: "nonexistent-agent",
				},
			}

			_, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should return error when cluster agent has empty Status.Url", func() {
			clusterAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-no-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, clusterAgent)).To(Succeed())

			subAgent := runtimev1alpha1.SubAgent{
				Name: "agent-no-url",
				AgentRef: &corev1.ObjectReference{
					Name: "agent-no-url",
				},
			}

			_, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no URL in its Status field"))
		})

		It("should return error when neither URL nor AgentRef is specified", func() {
			subAgent := runtimev1alpha1.SubAgent{
				Name: "invalid-subagent",
				// Neither Url nor AgentRef specified
			}

			_, err := reconciler.resolveSubAgentUrl(ctx, subAgent, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has neither url nor agentRef specified"))
		})
	})

	Describe("resolveAllSubAgents", func() {
		It("should resolve all subAgents successfully", func() {
			clusterAgent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-sub1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, clusterAgent1)).To(Succeed())

			clusterAgent1.Status.Url = "http://cluster-sub1.default.svc.cluster.local:8000/"
			Expect(k8sClient.Status().Update(ctx, clusterAgent1)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-resolve-all",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "cluster-sub1", AgentRef: &corev1.ObjectReference{Name: "cluster-sub1"}},
						{Name: "remote-sub", Url: "https://example.com/sub.json"},
					},
				},
			}

			resolved, err := reconciler.resolveAllSubAgents(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["cluster-sub1"]).To(Equal("http://cluster-sub1.default.svc.cluster.local:8000/"))
			Expect(resolved["remote-sub"]).To(Equal("https://example.com/sub.json"))
		})

		It("should collect all resolution errors", func() {
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-with-errors",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "missing-sub1", AgentRef: &corev1.ObjectReference{Name: "missing-sub1"}},
						{Name: "missing-sub2", AgentRef: &corev1.ObjectReference{Name: "missing-sub2"}},
					},
				},
			}

			_, err := reconciler.resolveAllSubAgents(ctx, parentAgent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve 2 subAgent(s)"))
			Expect(err.Error()).To(ContainSubstring("missing-sub1"))
			Expect(err.Error()).To(ContainSubstring("missing-sub2"))
		})

		It("should handle empty subAgents list", func() {
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-no-subs",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					SubAgents: []runtimev1alpha1.SubAgent{},
				},
			}

			resolved, err := reconciler.resolveAllSubAgents(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(BeEmpty())
		})
	})

	Describe("findAgentsReferencingSubAgent", func() {
		It("should identify parent agents referencing the changed agent", func() {
			subAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "referenced-subagent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, subAgent)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "referenced-subagent",
							AgentRef: &corev1.ObjectReference{
								Name:      "referenced-subagent",
								Namespace: "default",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingSubAgent(ctx, subAgent)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-agent"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should match with namespace defaulting", func() {
			subAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "subagent-with-defaulting",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, subAgent)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-with-defaulting",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "subagent-with-defaulting",
							AgentRef: &corev1.ObjectReference{
								Name: "subagent-with-defaulting",
							}}, // No namespace specified
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingSubAgent(ctx, subAgent)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-with-defaulting"))
		})

		It("should ignore remote URL references", func() {
			subAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "subagent-remote",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, subAgent)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-with-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "subagent-remote", Url: "https://example.com/sub.json"}, // Remote URL, not AgentRef
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingSubAgent(ctx, subAgent)
			Expect(requests).To(BeEmpty()) // Should not find the parent since it uses URL
		})

		It("should handle multiple parents referencing same subAgent", func() {
			subAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-subagent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, subAgent)).To(Succeed())

			parent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "shared-subagent", AgentRef: &corev1.ObjectReference{Name: "shared-subagent"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parent1)).To(Succeed())

			parent2 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent2",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "shared-subagent", AgentRef: &corev1.ObjectReference{Name: "shared-subagent"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parent2)).To(Succeed())

			requests := reconciler.findAgentsReferencingSubAgent(ctx, subAgent)
			Expect(requests).To(HaveLen(2))
			names := []string{requests[0].Name, requests[1].Name}
			Expect(names).To(ContainElements("parent1", "parent2"))
		})
	})
})
