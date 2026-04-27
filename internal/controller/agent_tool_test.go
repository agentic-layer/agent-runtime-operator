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

const (
	TestImage        = "test-image:latest"
	TestFramework    = "google-adk"
	DefaultNamespace = "default"
)

// createToolRoute creates a ToolRoute resource and optionally populates its status.Url.
// Pass an empty url to leave the status unset (useful for testing not-ready routes).
func createToolRoute(ctx context.Context, k8sClient client.Client, name, namespace, url string) *runtimev1alpha1.ToolRoute {
	tr := &runtimev1alpha1.ToolRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: runtimev1alpha1.ToolRouteSpec{
			ToolGatewayRef: &corev1.ObjectReference{Name: "tg"},
			Upstream: runtimev1alpha1.ToolRouteUpstream{
				External: &runtimev1alpha1.ExternalUpstream{Url: "https://upstream.example.com/mcp"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, tr)).To(Succeed())
	if url != "" {
		tr.Status.Url = url
		Expect(k8sClient.Status().Update(ctx, tr)).To(Succeed())
	}
	return tr
}

// createToolServer creates a ToolServer resource and optionally populates its status.Url.
// Pass an empty url to leave the status unset.
func createToolServer(ctx context.Context, k8sClient client.Client, name, namespace, url string) *runtimev1alpha1.ToolServer {
	ts := &runtimev1alpha1.ToolServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: runtimev1alpha1.ToolServerSpec{
			Protocol:      "mcp",
			TransportType: "http",
			Image:         TestImage,
		},
	}
	Expect(k8sClient.Create(ctx, ts)).To(Succeed())
	if url != "" {
		ts.Status.Url = url
		Expect(k8sClient.Status().Update(ctx, ts)).To(Succeed())
	}
	return ts
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

	toolRouteList := &runtimev1alpha1.ToolRouteList{}
	Expect(k8sClient.List(ctx, toolRouteList, &client.ListOptions{Namespace: namespace})).To(Succeed())
	for i := range toolRouteList.Items {
		_ = k8sClient.Delete(ctx, &toolRouteList.Items[i])
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

	Describe("resolveAllTools", func() {
		It("resolves a tool via ToolRoute.status.url", func() {
			createToolRoute(ctx, k8sClient, "tr-1", DefaultNamespace, "https://gw.local/r/default/tr-1/mcp")
			agent := createAgentWithTools(ctx, k8sClient, "a1", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-1"}},
			}})
			resolved, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(HaveKey("t1"))
			Expect(resolved["t1"].Url).To(Equal("https://gw.local/r/default/tr-1/mcp"))
		})

		It("returns aggregated error when a tool's ToolRoute is missing", func() {
			agent := createAgentWithTools(ctx, k8sClient, "a2", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "does-not-exist"}},
			}})
			_, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`tool "t1"`))
			Expect(err.Error()).To(ContainSubstring("failed to resolve"))
		})

		It("returns error when referenced ToolRoute has empty status.url", func() {
			createToolRoute(ctx, k8sClient, "tr-noready", DefaultNamespace, "")
			agent := createAgentWithTools(ctx, k8sClient, "a3", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-noready"}},
			}})
			_, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no URL"))
		})

		It("defaults ToolRoute namespace to the Agent's namespace", func() {
			createToolRoute(ctx, k8sClient, "tr-default-ns", DefaultNamespace, "https://gw/default/tr-default-ns")
			agent := createAgentWithTools(ctx, k8sClient, "a4", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-default-ns"}}, // no Namespace
			}})
			resolved, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["t1"].Url).To(Equal("https://gw/default/tr-default-ns"))
		})

		It("resolves a tool via upstream.toolServerRef", func() {
			createToolServer(ctx, k8sClient, "ts-direct", DefaultNamespace, "http://ts-direct.default.svc.cluster.local:8080/mcp")
			agent := createAgentWithTools(ctx, k8sClient, "a-tsref", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-direct"}},
			}})
			resolved, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["t1"].Url).To(Equal("http://ts-direct.default.svc.cluster.local:8080/mcp"))
		})

		It("returns error when referenced ToolServer has empty status.url", func() {
			createToolServer(ctx, k8sClient, "ts-nourl", DefaultNamespace, "")
			agent := createAgentWithTools(ctx, k8sClient, "a-ts-nourl", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-nourl"}},
			}})
			_, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`tool "t1"`))
			Expect(err.Error()).To(ContainSubstring("has no URL"))
		})

		It("returns error when referenced ToolServer does not exist", func() {
			agent := createAgentWithTools(ctx, k8sClient, "a-ts-missing", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "no-such-toolserver"}},
			}})
			_, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`tool "t1"`))
			Expect(err.Error()).To(ContainSubstring("failed to resolve"))
		})

		It("defaults ToolServerRef namespace to the Agent's namespace", func() {
			createToolServer(ctx, k8sClient, "ts-defns", DefaultNamespace, "http://ts-defns.default.svc.cluster.local:8080/mcp")
			agent := createAgentWithTools(ctx, k8sClient, "a-ts-defns", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-defns"}}, // no Namespace set
			}})
			resolved, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["t1"].Url).To(Equal("http://ts-defns.default.svc.cluster.local:8080/mcp"))
		})

		It("resolves a tool via upstream.external.url directly", func() {
			agent := createAgentWithTools(ctx, k8sClient, "a-ext", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{External: &runtimev1alpha1.ExternalUpstream{Url: "https://mcp.example.com/mcp"}},
			}})
			resolved, err := reconciler.resolveAllTools(ctx, agent)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved["t1"].Url).To(Equal("https://mcp.example.com/mcp"))
		})
	})

	Describe("findAgentsReferencingToolRoute", func() {
		It("enqueues an agent that references the changed ToolRoute by name and namespace", func() {
			tr := createToolRoute(ctx, k8sClient, "tr-watch", DefaultNamespace, "https://gw/watch")
			createAgentWithTools(ctx, k8sClient, "a-watch", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-watch"}},
			}})

			requests := reconciler.findAgentsReferencingToolRoute(ctx, tr)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("a-watch"))
			Expect(requests[0].Namespace).To(Equal(DefaultNamespace))
		})

		It("does not enqueue agents when namespace differs", func() {
			createTestNamespace(ctx, k8sClient, "other-ns")
			trOther := createToolRoute(ctx, k8sClient, "tr-ns", "other-ns", "https://gw/other")
			// Agent in default references "tr-ns" with no explicit namespace → defaults to "default"
			createAgentWithTools(ctx, k8sClient, "a-ns", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-ns"}},
			}})

			requests := reconciler.findAgentsReferencingToolRoute(ctx, trOther)
			Expect(requests).To(BeEmpty())
		})

		It("enqueues agent only once even when multiple tools reference the same ToolRoute", func() {
			tr := createToolRoute(ctx, k8sClient, "tr-multi", DefaultNamespace, "https://gw/multi")
			createAgentWithTools(ctx, k8sClient, "a-multi", []runtimev1alpha1.AgentTool{
				{Name: "t1", Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-multi"}}},
				{Name: "t2", Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-multi"}}},
			})

			requests := reconciler.findAgentsReferencingToolRoute(ctx, tr)
			Expect(requests).To(HaveLen(1))
		})

		It("defaults ToolRouteRef namespace to agent namespace for matching", func() {
			tr := createToolRoute(ctx, k8sClient, "tr-defns", DefaultNamespace, "https://gw/defns")
			// Tool has explicit namespace matching the route's namespace
			createAgentWithTools(ctx, k8sClient, "a-defns", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-defns", Namespace: DefaultNamespace}},
			}})

			requests := reconciler.findAgentsReferencingToolRoute(ctx, tr)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("a-defns"))
		})
	})

	Describe("findAgentsReferencingToolServer", func() {
		It("enqueues an agent whose upstream.toolServerRef matches the changed ToolServer", func() {
			ts := createToolServer(ctx, k8sClient, "ts-watch", DefaultNamespace, "http://ts-watch.default.svc.cluster.local:8080/mcp")
			createAgentWithTools(ctx, k8sClient, "a-ts-watch", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-watch"}},
			}})

			requests := reconciler.findAgentsReferencingToolServer(ctx, ts)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("a-ts-watch"))
			Expect(requests[0].Namespace).To(Equal(DefaultNamespace))
		})

		It("does not enqueue agents that use upstream.toolRouteRef, not toolServerRef", func() {
			ts := createToolServer(ctx, k8sClient, "ts-unref", DefaultNamespace, "")
			createToolRoute(ctx, k8sClient, "tr-1-ts", DefaultNamespace, "https://gw/tr-1-ts")
			createAgentWithTools(ctx, k8sClient, "a-route-only", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolRouteRef: &corev1.ObjectReference{Name: "tr-1-ts"}},
			}})

			requests := reconciler.findAgentsReferencingToolServer(ctx, ts)
			Expect(requests).To(BeEmpty())
		})

		It("does not enqueue agents when namespace differs", func() {
			createTestNamespace(ctx, k8sClient, "ts-other-ns")
			tsOther := createToolServer(ctx, k8sClient, "ts-ns", "ts-other-ns", "")
			createAgentWithTools(ctx, k8sClient, "a-ts-ns", []runtimev1alpha1.AgentTool{{
				Name:     "t1",
				Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-ns"}}, // defaults to "default"
			}})

			requests := reconciler.findAgentsReferencingToolServer(ctx, tsOther)
			Expect(requests).To(BeEmpty())
		})

		It("enqueues agent only once when multiple tools reference the same ToolServer", func() {
			ts := createToolServer(ctx, k8sClient, "ts-multi", DefaultNamespace, "http://ts-multi.default.svc.cluster.local:8080/mcp")
			createAgentWithTools(ctx, k8sClient, "a-ts-multi", []runtimev1alpha1.AgentTool{
				{Name: "t1", Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-multi"}}},
				{Name: "t2", Upstream: runtimev1alpha1.AgentToolUpstream{ToolServerRef: &corev1.ObjectReference{Name: "ts-multi"}}},
			})

			requests := reconciler.findAgentsReferencingToolServer(ctx, ts)
			Expect(requests).To(HaveLen(1))
		})
	})
})
