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

var toolGatewaySpec = runtimev1alpha1.ToolGatewaySpec{
	ToolGatewayClassName: "default",
}

var _ = Describe("ToolServer ToolGateway Resolution", func() {
	ctx := context.Background()
	var reconciler *ToolServerReconciler

	BeforeEach(func() {
		reconciler = &ToolServerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
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
		// Clean up ALL tool servers across all namespaces to avoid test pollution
		// This is important because other test files (like agent_tool_test.go) may create
		// tool servers in various namespaces that can interfere with our tests
		allToolServerList := &runtimev1alpha1.ToolServerList{}
		_ = k8sClient.List(ctx, allToolServerList)
		for i := range allToolServerList.Items {
			_ = k8sClient.Delete(ctx, &allToolServerList.Items[i])
		}

		// Clean up ALL ToolGateways across all namespaces
		allToolGatewayList := &runtimev1alpha1.ToolGatewayList{}
		_ = k8sClient.List(ctx, allToolGatewayList)
		for i := range allToolGatewayList.Items {
			_ = k8sClient.Delete(ctx, &allToolGatewayList.Items[i])
		}
	})

	Describe("resolveToolGateway", func() {
		It("should resolve explicit ToolGatewayRef with specified namespace", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server with explicit ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "test-gateway",
						Namespace: defaultToolGatewayNamespace,
					},
				},
			}

			resolvedGateway, err := reconciler.resolveToolGateway(ctx, toolServer)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).NotTo(BeNil())
			Expect(resolvedGateway.Name).To(Equal("test-gateway"))
			Expect(resolvedGateway.Namespace).To(Equal(defaultToolGatewayNamespace))
		})

		It("should default to tool server's namespace when ToolGatewayRef namespace is not specified", func() {
			// Create ToolGateway in default namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "default",
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server with ToolGatewayRef without namespace
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name: "test-gateway",
						// Namespace not specified - should default to tool server's namespace
					},
				},
			}

			resolvedGateway, err := reconciler.resolveToolGateway(ctx, toolServer)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).NotTo(BeNil())
			Expect(resolvedGateway.Name).To(Equal("test-gateway"))
			Expect(resolvedGateway.Namespace).To(Equal("default"))
		})

		It("should resolve default ToolGateway when no ToolGatewayRef is specified", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace (default location)
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server without ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					// No ToolGatewayRef specified
				},
			}

			resolvedGateway, err := reconciler.resolveToolGateway(ctx, toolServer)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).NotTo(BeNil())
			Expect(resolvedGateway.Name).To(Equal("default-gateway"))
			Expect(resolvedGateway.Namespace).To(Equal(defaultToolGatewayNamespace))
		})

		It("should return nil when no ToolGatewayRef specified and no default gateway exists", func() {
			// Create tool server without ToolGatewayRef and no default gateway exists
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					// No ToolGatewayRef specified and no default exists
				},
			}

			resolvedGateway, err := reconciler.resolveToolGateway(ctx, toolServer)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).To(BeNil())
		})

		It("should return error when explicit ToolGatewayRef points to non-existent gateway", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "non-existent",
						Namespace: "default",
					},
				},
			}

			resolvedGateway, err := reconciler.resolveToolGateway(ctx, toolServer)
			Expect(err).To(HaveOccurred())
			Expect(resolvedGateway).To(BeNil())
		})
	})

	Describe("resolveDefaultToolGateway", func() {
		It("should find ToolGateway in tool-gateway namespace", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			resolvedGateway, err := reconciler.resolveDefaultToolGateway(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).NotTo(BeNil())
			Expect(resolvedGateway.Name).To(Equal("default-gateway"))
			Expect(resolvedGateway.Namespace).To(Equal(defaultToolGatewayNamespace))
		})

		It("should return nil when no ToolGateway exists in tool-gateway namespace", func() {
			resolvedGateway, err := reconciler.resolveDefaultToolGateway(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedGateway).To(BeNil())
		})
	})

	Describe("findToolServersReferencingToolGateway", func() {
		It("should identify tool servers with explicit ToolGatewayRef", func() {
			// Create or get necessary namespaces
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server with explicit ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "test-gateway",
						Namespace: defaultToolGatewayNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			requests := reconciler.findToolServersReferencingToolGateway(ctx, toolGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("test-toolserver"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should match with namespace defaulting in ToolGatewayRef", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server with ToolGatewayRef without namespace (defaults to tool server's namespace)
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name: "test-gateway",
						// Namespace not specified - defaults to tool server's namespace (tool-gateway)
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			requests := reconciler.findToolServersReferencingToolGateway(ctx, toolGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("test-toolserver"))
			Expect(requests[0].Namespace).To(Equal(defaultToolGatewayNamespace))
		})

		It("should identify tool servers using default gateway resolution", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server without ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					// No ToolGatewayRef - would use default resolution
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			requests := reconciler.findToolServersReferencingToolGateway(ctx, toolGateway)
			Expect(requests).To(HaveLen(1))
			Expect(requests[0].Name).To(Equal("test-toolserver"))
			Expect(requests[0].Namespace).To(Equal("default"))
		})

		It("should not match tool servers with default resolution when gateway is in non-default namespace", func() {
			// Create or get other-namespace
			createNamespaceIfNotExists("other-namespace")

			// Create ToolGateway in other-namespace (not the default tool-gateway namespace)
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-gateway",
					Namespace: "other-namespace",
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server without ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					// No ToolGatewayRef - would not match non-default gateway
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			requests := reconciler.findToolServersReferencingToolGateway(ctx, toolGateway)
			Expect(requests).To(BeEmpty())
		})

		It("should handle multiple tool servers correctly", func() {
			// Create or get necessary namespaces
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway in tool-gateway namespace
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-gateway",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create tool server with explicit reference
			toolServer1 := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "explicit-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "shared-gateway",
						Namespace: defaultToolGatewayNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer1)).To(Succeed())

			// Create tool server without reference (uses default)
			toolServer2 := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					// No ToolGatewayRef
				},
			}
			Expect(k8sClient.Create(ctx, toolServer2)).To(Succeed())

			requests := reconciler.findToolServersReferencingToolGateway(ctx, toolGateway)
			Expect(requests).To(HaveLen(2))
			// Verify both tool servers are included
			toolServerNames := []string{requests[0].Name, requests[1].Name}
			Expect(toolServerNames).To(ConsistOf("explicit-toolserver", "default-toolserver"))
		})
	})

	Describe("GatewayUrl Population", func() {
		It("should populate GatewayUrl when ToolGateway has Status.Url", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway with Status.Url
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-with-url",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Set the gateway's status URL (simulating what a gateway controller would do)
			toolGateway.Status.Url = "http://tool-gateway.tool-gateway.svc.cluster.local:8080"
			Expect(k8sClient.Status().Update(ctx, toolGateway)).To(Succeed())

			// Create ToolServer with explicit ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					Path:          "/mcp",
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "gateway-with-url",
						Namespace: defaultToolGatewayNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			// Simulate the reconciler updating the status
			err := reconciler.updateToolServerStatusReady(ctx, toolServer, toolGateway)
			Expect(err).NotTo(HaveOccurred())

			// Verify GatewayUrl is populated correctly
			expectedGatewayUrl := "http://tool-gateway.tool-gateway.svc.cluster.local:8080" +
				"/toolserver/default/test-toolserver/mcp"
			Expect(toolServer.Status.GatewayUrl).To(Equal(expectedGatewayUrl))
			Expect(toolServer.Status.ToolGatewayRef).NotTo(BeNil())
			Expect(toolServer.Status.ToolGatewayRef.Name).To(Equal("gateway-with-url"))
		})

		It("should not populate GatewayUrl when ToolGateway Status.Url is empty", func() {
			// Create or get tool-gateway namespace
			createNamespaceIfNotExists(defaultToolGatewayNamespace)

			// Create ToolGateway without Status.Url
			toolGateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-without-url",
					Namespace: defaultToolGatewayNamespace,
				},
				Spec: toolGatewaySpec,
			}
			Expect(k8sClient.Create(ctx, toolGateway)).To(Succeed())

			// Create ToolServer with explicit ToolGatewayRef
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver-no-gateway-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					Path:          "/mcp",
					ToolGatewayRef: &corev1.ObjectReference{
						Name:      "gateway-without-url",
						Namespace: defaultToolGatewayNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			// Simulate the reconciler updating the status
			err := reconciler.updateToolServerStatusReady(ctx, toolServer, toolGateway)
			Expect(err).NotTo(HaveOccurred())

			// Verify GatewayUrl is NOT populated when gateway URL is empty
			Expect(toolServer.Status.GatewayUrl).To(BeEmpty())
			Expect(toolServer.Status.ToolGatewayRef).NotTo(BeNil())
			Expect(toolServer.Status.ToolGatewayRef.Name).To(Equal("gateway-without-url"))
		})

		It("should clear GatewayUrl when ToolGateway is nil", func() {
			// Create ToolServer
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver-no-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
					Path:          "/mcp",
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			// Simulate the reconciler updating the status without a gateway
			err := reconciler.updateToolServerStatusReady(ctx, toolServer, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify GatewayUrl is empty and ToolGatewayRef is nil
			Expect(toolServer.Status.GatewayUrl).To(BeEmpty())
			Expect(toolServer.Status.ToolGatewayRef).To(BeNil())
		})

		It("should clear GatewayUrl when status is set to not ready", func() {
			// Create ToolServer with pre-existing GatewayUrl
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-toolserver-not-ready",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					Image:         "test-image:latest",
					TransportType: "http",
					Port:          8080,
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			// Set pre-existing status
			toolServer.Status.GatewayUrl = "http://old-gateway-url/path"
			toolServer.Status.ToolGatewayRef = &corev1.ObjectReference{
				Name: "old-gateway",
			}
			Expect(k8sClient.Status().Update(ctx, toolServer)).To(Succeed())

			// Simulate the reconciler setting status to not ready
			err := reconciler.updateToolServerStatusNotReady(ctx, toolServer, "TestReason", "Test message")
			Expect(err).NotTo(HaveOccurred())

			// Verify GatewayUrl and ToolGatewayRef are cleared
			Expect(toolServer.Status.GatewayUrl).To(BeEmpty())
			Expect(toolServer.Status.ToolGatewayRef).To(BeNil())
		})
	})
})
