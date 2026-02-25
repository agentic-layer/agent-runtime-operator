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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	TestProtocol      = "mcp"
	TestTransportHTTP = "http"
	TestImage         = "test-image:latest"
	TestFramework     = "google-adk"
	DefaultNamespace  = "default"
)

func createToolServer(ctx context.Context, k8sClient client.Client, name, namespace string) *runtimev1alpha1.ToolServer {
	toolServer := &runtimev1alpha1.ToolServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: runtimev1alpha1.ToolServerSpec{
			Protocol:      TestProtocol,
			TransportType: TestTransportHTTP,
			Image:         TestImage,
		},
	}
	Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())
	return toolServer
}

func createToolServerWithURL(ctx context.Context, k8sClient client.Client, name, namespace string) *runtimev1alpha1.ToolServer {
	toolServer := createToolServer(ctx, k8sClient, name, namespace)

	url := generateServiceURL(name, namespace)
	toolServer.Status.Url = url
	Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

	return toolServer
}

func generateServiceURL(name, namespace string) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", name, namespace)
}

func createAgentWithToolServerRef(ctx context.Context, k8sClient client.Client, agentName, toolServerName, toolServerNamespace string) {
	var toolServerRef *corev1.ObjectReference
	if toolServerNamespace == "" {
		toolServerRef = &corev1.ObjectReference{
			Name: toolServerName,
		}
	} else {
		toolServerRef = &corev1.ObjectReference{
			Name:      toolServerName,
			Namespace: toolServerNamespace,
		}
	}

	agent := &runtimev1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentName,
			Namespace: DefaultNamespace,
		},
		Spec: runtimev1alpha1.AgentSpec{
			Framework: TestFramework,
			Image:     TestImage,
			Tools: []runtimev1alpha1.AgentTool{
				{
					Name:          "test-tool",
					ToolServerRef: toolServerRef,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, agent)).To(Succeed())
}

func createAgentWithTools(ctx context.Context, k8sClient client.Client, name string, tools []runtimev1alpha1.AgentTool) *runtimev1alpha1.Agent {
	agent := &runtimev1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: DefaultNamespace,
		},
		Spec: runtimev1alpha1.AgentSpec{
			Framework: TestFramework,
			Image:     TestImage,
			Tools:     tools,
		},
	}
	Expect(k8sClient.Create(ctx, agent)).To(Succeed())
	return agent
}

func createTestNamespace(ctx context.Context, k8sClient client.Client, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() {
		_ = k8sClient.Delete(ctx, ns)
	})
}

func cleanupResourcesInNamespace(ctx context.Context, k8sClient client.Client, namespace string) {
	agentList := &runtimev1alpha1.AgentList{}
	Expect(k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: namespace})).To(Succeed())
	for i := range agentList.Items {
		_ = k8sClient.Delete(ctx, &agentList.Items[i])
	}

	toolServerList := &runtimev1alpha1.ToolServerList{}
	Expect(k8sClient.List(ctx, toolServerList, &client.ListOptions{Namespace: namespace})).To(Succeed())
	for i := range toolServerList.Items {
		_ = k8sClient.Delete(ctx, &toolServerList.Items[i])
	}
}

var _ = Describe("Agent Tool", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		// Set POD_NAMESPACE for tests
		Expect(os.Setenv("POD_NAMESPACE", "default")).To(Succeed())
	})

	AfterEach(func() {
		cleanupResourcesInNamespace(ctx, k8sClient, DefaultNamespace)
	})

	Describe("resolveToolServerUrl", func() {
		It("should resolve ToolServer with explicit namespace", func() {
			By("creating a test namespace")
			createTestNamespace(ctx, k8sClient, "test-ns-toolserver-explicit")

			By("creating a ToolServer in the test namespace")
			toolServer := createToolServerWithURL(ctx, k8sClient, "cross-ns-toolserver", "test-ns-toolserver-explicit")

			By("resolving a tool with explicit namespace reference")
			tool := runtimev1alpha1.AgentTool{
				Name: "cross-ns-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name:      toolServer.Name,
					Namespace: toolServer.Namespace,
				},
			}

			By("verifying the URL is resolved correctly")
			url, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(toolServer.Status.Url))
		})

		It("should return error when ToolServer doesn't exist", func() {
			By("creating a tool reference to a non-existent ToolServer")
			tool := runtimev1alpha1.AgentTool{
				Name: "missing-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: "nonexistent-toolserver",
				},
			}

			By("verifying an error is returned")
			_, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve ToolServer"))
			Expect(err.Error()).To(ContainSubstring("nonexistent-toolserver"))
		})

		It("should return direct URL when provided", func() {
			By("creating a tool with a direct URL")
			tool := runtimev1alpha1.AgentTool{
				Name: "remote-tool",
				Url:  "https://mcp.example.com/tools",
			}

			By("verifying the direct URL is returned")
			url, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://mcp.example.com/tools"))
		})

		It("should return error when neither toolServerRef nor url is specified", func() {
			By("creating a tool without URL or ToolServerRef")
			tool := runtimev1alpha1.AgentTool{
				Name: "incomplete-tool",
			}

			By("verifying an error is returned")
			_, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has neither url nor toolServerRef specified"))
		})

		It("should prefer GatewayUrl over Url when both are present", func() {
			By("creating a ToolServer with both Url and GatewayUrl")
			toolServer := createToolServer(ctx, k8sClient, "gateway-toolserver", DefaultNamespace)
			toolServer.Status.Url = "http://gateway-toolserver.default.svc.cluster.local:8080"
			toolServer.Status.GatewayUrl = "http://tool-gateway.tool-gateway.svc.cluster.local:8080/default/gateway-toolserver"
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			By("creating a tool reference to the ToolServer")
			tool := runtimev1alpha1.AgentTool{
				Name: "gateway-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: toolServer.Name,
				},
			}

			By("verifying GatewayUrl is preferred over Url")
			url, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(toolServer.Status.GatewayUrl))
		})

		It("should fall back to Url when GatewayUrl is not set", func() {
			By("creating a ToolServer with only Url (no GatewayUrl)")
			toolServer := createToolServerWithURL(ctx, k8sClient, "direct-toolserver", DefaultNamespace)

			By("creating a tool reference to the ToolServer")
			tool := runtimev1alpha1.AgentTool{
				Name: "direct-tool",
				ToolServerRef: &corev1.ObjectReference{
					Name: toolServer.Name,
				},
			}

			By("verifying Url is used when GatewayUrl is empty")
			url, err := reconciler.resolveToolServerUrl(ctx, tool, DefaultNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(toolServer.Status.Url))
		})
	})

	Describe("resolveAllTools", func() {
		It("should resolve all tools successfully", func() {
			By("creating the first ToolServer")
			toolServer1 := createToolServerWithURL(ctx, k8sClient, "toolserver1", DefaultNamespace)

			By("creating the second ToolServer")
			toolServer2 := createToolServerWithURL(ctx, k8sClient, "toolserver2", DefaultNamespace)

			By("creating an agent with tools referencing both ToolServers")
			parentAgent := createAgentWithTools(ctx, k8sClient, "parent-resolve-all", []runtimev1alpha1.AgentTool{
				{Name: "tool1", ToolServerRef: &corev1.ObjectReference{Name: toolServer1.Name}},
				{Name: "tool2", ToolServerRef: &corev1.ObjectReference{Name: toolServer2.Name}},
			})

			By("resolving all tools for the agent")
			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)

			By("verifying both tools are resolved correctly")
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["tool1"].Url).To(Equal(toolServer1.Status.Url))
			Expect(resolved["tool2"].Url).To(Equal(toolServer2.Status.Url))
		})

		It("should collect all resolution errors", func() {
			By("creating an agent with references to non-existent ToolServers")
			parentAgent := createAgentWithTools(ctx, k8sClient, "parent-with-errors", []runtimev1alpha1.AgentTool{
				{Name: "missing-tool1", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver1"}},
				{Name: "missing-tool2", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver2"}},
			})

			By("verifying all resolution errors are collected")
			_, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve 2 tool(s)"))
			Expect(err.Error()).To(ContainSubstring("missing-tool1"))
			Expect(err.Error()).To(ContainSubstring("missing-tool2"))
		})

		It("should handle empty tools list", func() {
			By("creating an agent with no tools")
			parentAgent := createAgentWithTools(ctx, k8sClient, "parent-no-tools", []runtimev1alpha1.AgentTool{})

			By("verifying empty result is returned")
			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(BeEmpty())
		})

		It("should return partial results when some tools fail", func() {
			By("creating a working ToolServer")
			workingToolServer := createToolServerWithURL(ctx, k8sClient, "working-toolserver", DefaultNamespace)

			By("creating an agent with multiple tools (one working, one missing)")
			parentAgent := createAgentWithTools(ctx, k8sClient, "parent-mixed-tools", []runtimev1alpha1.AgentTool{
				{Name: "working-tool", ToolServerRef: &corev1.ObjectReference{Name: workingToolServer.Name}},
				{Name: "missing-tool", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver"}},
			})

			By("resolving all tools for the agent")
			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)

			By("verifying an error is returned for the missing tool")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve 1 tool(s)"))
			Expect(err.Error()).To(ContainSubstring("missing-tool"))

			By("verifying partial results contain the working tool")
			Expect(resolved).To(HaveLen(1))
			Expect(resolved["working-tool"].Url).To(Equal(workingToolServer.Status.Url))
		})

		It("should resolve mixed toolServerRef and direct URLs", func() {
			By("creating a ToolServer")
			toolServer := createToolServerWithURL(ctx, k8sClient, "local-toolserver", DefaultNamespace)

			By("creating an agent with mixed tools (ToolServerRef and direct URL)")
			parentAgent := createAgentWithTools(ctx, k8sClient, "parent-mixed-types", []runtimev1alpha1.AgentTool{
				{Name: "local-tool", ToolServerRef: &corev1.ObjectReference{Name: toolServer.Name}},
				{Name: "remote-tool", Url: "https://mcp.example.com/tools"},
			})

			By("resolving all tools for the agent")
			resolved, err := reconciler.resolveAllTools(ctx, parentAgent)

			By("verifying both tool types are resolved correctly")
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveLen(2))
			Expect(resolved["local-tool"].Url).To(Equal(toolServer.Status.Url))
			Expect(resolved["remote-tool"].Url).To(Equal("https://mcp.example.com/tools"))
		})
	})

	Describe("findAgentsReferencingToolServer", func() {
		It("should identify agents referencing the changed ToolServer", func() {
			By("creating a ToolServer")
			toolServer := createToolServerWithURL(ctx, k8sClient, "referenced-toolserver", DefaultNamespace)

			By("creating an agent that references the ToolServer")
			createAgentWithToolServerRef(ctx, k8sClient, "parent-agent", toolServer.Name, toolServer.Namespace)

			By("finding agents that reference the ToolServer")
			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)

			By("verifying the agent is found")
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-agent"))
			Expect(requests[0].Namespace).To(Equal(DefaultNamespace))
		})

		It("should find agents with empty toolServerRef namespace", func() {
			By("creating a ToolServer")
			toolServer := createToolServerWithURL(ctx, k8sClient, "toolserver-with-defaulting", DefaultNamespace)

			By("creating an agent that references the ToolServer without namespace")
			createAgentWithToolServerRef(ctx, k8sClient, "parent-with-defaulting", toolServer.Name, "")

			By("finding agents that reference the ToolServer")
			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)

			By("verifying the agent is found when toolServerRef.namespace is empty")
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-with-defaulting"))
		})

		It("should handle multiple agents referencing same ToolServer", func() {
			By("creating a shared ToolServer")
			toolServer := createToolServerWithURL(ctx, k8sClient, "shared-toolserver", DefaultNamespace)

			By("creating the first agent referencing the ToolServer")
			createAgentWithToolServerRef(ctx, k8sClient, "parent1", toolServer.Name, "")

			By("creating the second agent referencing the same ToolServer")
			createAgentWithToolServerRef(ctx, k8sClient, "parent2", toolServer.Name, "")

			By("finding all agents that reference the ToolServer")
			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)

			By("verifying both agents are found")
			Expect(requests).To(HaveLen(2))
			names := []string{requests[0].Name, requests[1].Name}
			Expect(names).To(ContainElements("parent1", "parent2"))
		})

		It("should handle cross-namespace references", func() {
			By("creating a test namespace")
			createTestNamespace(ctx, k8sClient, "test-ns-cross-ref")

			By("creating a ToolServer in the test namespace")
			toolServer := createToolServerWithURL(ctx, k8sClient, "cross-ns-toolserver", "test-ns-cross-ref")

			By("creating an agent in a different namespace that references the ToolServer")
			createAgentWithToolServerRef(ctx, k8sClient, "parent-cross-ns", toolServer.Name, toolServer.Namespace)

			By("finding agents that reference the ToolServer")
			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)

			By("verifying the cross-namespace agent is found")
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("parent-cross-ns"))
			Expect(requests[0].Namespace).To(Equal(DefaultNamespace))
		})

		It("should return empty list when no agents reference the ToolServer", func() {
			By("creating a ToolServer with no references")
			toolServer := createToolServer(ctx, k8sClient, "unreferenced-toolserver", DefaultNamespace)

			By("verifying no agents are found")
			requests := reconciler.findAgentsReferencingToolServer(ctx, toolServer)
			Expect(requests).To(BeEmpty())
		})
	})
})
