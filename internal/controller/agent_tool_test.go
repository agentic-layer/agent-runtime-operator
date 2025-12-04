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

var _ = Describe("Agent Tool", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		// Clean up all agents and toolservers after each test
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}

		toolServerList := &runtimev1alpha1.ToolServerList{}
		Expect(k8sClient.List(ctx, toolServerList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range toolServerList.Items {
			_ = k8sClient.Delete(ctx, &toolServerList.Items[i])
		}
	})

	Describe("resolveToolServerUrl", func() {
		It("should resolve ToolServer with explicit namespace", func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-toolserver-explicit",
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, ns)
			})

			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-toolserver",
					Namespace: "test-ns-toolserver-explicit",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://cross-ns-toolserver.test-ns-toolserver-explicit.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			tool := runtimev1alpha1.AgentTool{
				Name: "cross-ns-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name:      "cross-ns-toolserver",
					Namespace: "test-ns-toolserver-explicit",
				},
			}

			url, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://cross-ns-toolserver.test-ns-toolserver-explicit.svc.cluster.local:8080"))
		})

		It("should default to parent agent's namespace", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://local-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			tool := runtimev1alpha1.AgentTool{
				Name: "local-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: "local-toolserver",
					// Namespace is omitted to test defaulting
				},
			}

			url, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://local-toolserver.default.svc.cluster.local:8080"))
		})

		It("should return error when ToolServer doesn't exist", func() {
			tool := runtimev1alpha1.AgentTool{
				Name: "missing-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: "nonexistent-toolserver",
				},
			}

			_, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve ToolServer"))
			Expect(err.Error()).To(ContainSubstring("nonexistent-toolserver"))
		})

		It("should return error when ToolServer has empty Status.Url", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stdio-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "stdio",
					Image:         "test-image:latest",
					Command:       []string{"/bin/tool"},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			tool := runtimev1alpha1.AgentTool{
				Name: "stdio-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: "stdio-toolserver",
				},
			}

			_, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no URL in its Status field"))
			Expect(err.Error()).To(ContainSubstring("stdio"))
		})

		It("should return direct URL when provided", func() {
			tool := runtimev1alpha1.AgentTool{
				Name: "remote-tool",
				Url:  "https://mcp.example.com/tools",
			}

			url, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://mcp.example.com/tools"))
		})

		It("should return error when neither toolServerRef nor url is specified", func() {
			tool := runtimev1alpha1.AgentTool{
				Name: "incomplete-tool",
			}

			_, err := reconciler.resolveToolServerUrl(ctx, tool, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has neither url nor toolServerRef specified"))
		})
	})

	Describe("resolveAllTools", func() {
		It("should resolve all tools successfully", func() {
			toolServer1 := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "toolserver1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer1)).To(Succeed())

			toolServer1.Status.Url = "http://toolserver1.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer1)).To(Succeed())

			toolServer2 := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "toolserver2",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer2)).To(Succeed())

			toolServer2.Status.Url = "http://toolserver2.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer2)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-resolve-all",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "tool1", ToolServerRef: &corev1.ObjectReference{Name: "toolserver1"}},
						{Name: "tool2", ToolServerRef: &corev1.ObjectReference{Name: "toolserver2"}},
					},
				},
			}

			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["tool1"]).To(Equal("http://toolserver1.default.svc.cluster.local:8080"))
			Expect(resolved["tool2"]).To(Equal("http://toolserver2.default.svc.cluster.local:8080"))
		})

		It("should collect all resolution errors", func() {
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-with-errors",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "missing-tool1", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver1"}},
						{Name: "missing-tool2", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver2"}},
					},
				},
			}

			_, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve 2 tool(s)"))
			Expect(err.Error()).To(ContainSubstring("missing-tool1"))
			Expect(err.Error()).To(ContainSubstring("missing-tool2"))
		})

		It("should handle empty tools list", func() {
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-no-tools",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{},
				},
			}

			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(BeEmpty())
		})

		It("should return partial results when some tools fail", func() {
			// Create one working ToolServer
			workingToolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "working-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, workingToolServer)).To(Succeed())
			workingToolServer.Status.Url = "http://working-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, workingToolServer)).To(Succeed())

			// Agent with mixed tools (one working, one missing)
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-mixed-tools",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "working-tool", ToolServerRef: &corev1.ObjectReference{Name: "working-toolserver"}},
						{Name: "missing-tool", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver"}},
					},
				},
			}

			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)

			// Should return error but also partial results
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve 1 tool(s)"))
			Expect(err.Error()).To(ContainSubstring("missing-tool"))

			// Verify partial results contain the working tool
			Expect(resolved).To(HaveLen(1))
			Expect(resolved["working-tool"]).To(Equal("http://working-toolserver.default.svc.cluster.local:8080"))
		})

		It("should resolve mixed toolServerRef and direct URLs", func() {
			// Create one ToolServer
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())
			toolServer.Status.Url = "http://local-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			// Agent with mixed tools (ToolServerRef and direct URL)
			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-mixed-types",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "local-tool", ToolServerRef: &corev1.ObjectReference{Name: "local-toolserver"}},
						{Name: "remote-tool", Url: "https://mcp.example.com/tools"},
					},
				},
			}

			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["local-tool"]).To(Equal("http://local-toolserver.default.svc.cluster.local:8080"))
			Expect(resolved["remote-tool"]).To(Equal("https://mcp.example.com/tools"))
		})
	})

	Describe("findAgentsReferencingToolServer", func() {
		It("should identify agents referencing the changed ToolServer", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "referenced-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://referenced-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{
							Name: "referenced-tool",
							ToolServerRef: &corev1.ObjectReference{
								Name:      "referenced-toolserver",
								Namespace: "default",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-agent"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should match with namespace defaulting", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "toolserver-with-defaulting",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://toolserver-with-defaulting.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-with-defaulting",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{
							Name: "tool-with-defaulting",
							ToolServerRef: &corev1.ObjectReference{
								Name: "toolserver-with-defaulting",
								// No namespace specified
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-with-defaulting"))
		})

		It("should handle multiple agents referencing same ToolServer", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://shared-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			parent1 := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent1",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "shared-tool", ToolServerRef: &corev1.ObjectReference{Name: "shared-toolserver"}},
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
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "shared-tool", ToolServerRef: &corev1.ObjectReference{Name: "shared-toolserver"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parent2)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(HaveLen(2))
			names := []string{requests[0].Name, requests[1].Name}
			Expect(names).To(ContainElements("parent1", "parent2"))
		})

		It("should handle cross-namespace references", func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns-cross-ref",
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, ns)
			})

			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-toolserver",
					Namespace: "test-ns-cross-ref",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://cross-ns-toolserver.test-ns-cross-ref.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			parentAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-cross-ns",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{
							Name: "cross-ns-tool",
							ToolServerRef: &corev1.ObjectReference{
								Name:      "cross-ns-toolserver",
								Namespace: "test-ns-cross-ref",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, parentAgent)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-cross-ns"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should return empty list when no agents reference the ToolServer", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unreferenced-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(BeEmpty())
		})

		It("should skip agents with tools using direct URLs", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			toolServer.Status.Url = "http://some-toolserver.default.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			// Agent with only direct URL tools (no ToolServerRef)
			agentWithDirectUrl := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-direct-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "remote-tool", Url: "https://mcp.example.com/tools"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentWithDirectUrl)).To(Succeed())

			// Agent with mixed tools (should only match if it has the ToolServerRef)
			agentMixed := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-mixed",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "remote-tool", Url: "https://mcp.example.com/tools"},
						{Name: "local-tool", ToolServerRef: &corev1.ObjectReference{Name: "some-toolserver"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agentMixed)).To(Succeed())

			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			// Should only return agent-mixed (which has a ToolServerRef), not agent-direct-url
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("agent-mixed"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})
	})
})
