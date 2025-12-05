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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Controller", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		// Clean up all agents in the default namespace after each test
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}
	})

	Describe("Reconcile", func() {
		It("should successfully reconcile a basic agent", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-basic-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "image:tag",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-basic-agent",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was created
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-basic-agent", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			// Verify service was created
			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-basic-agent", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Spec.Ports).To(HaveLen(1))
		})

		It("should return nil when agent is not found", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent-agent",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when subAgent cannot be resolved", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-subagent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "missing-sub", AgentRef: &corev1.ObjectReference{Name: "missing-sub"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-missing-subagent",
					Namespace: "default",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve"))
		})

		It("should fail when toolserver cannot be resolved", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "missing-tool", ToolServerRef: &corev1.ObjectReference{Name: "missing-toolserver"}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-missing-toolserver",
					Namespace: "default",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve"))
		})

		It("should reconcile successfully with AiGateway and update status", func() {
			// Create AiGateway in ai-gateway namespace
			aiGatewayNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ai-gateway",
				},
			}
			err := k8sClient.Create(ctx, aiGatewayNs)
			if err != nil && !errors.IsAlreadyExists(err) {
				Fail("Failed to create ai-gateway namespace: " + err.Error())
			}

			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-reconcile-gateway",
					Namespace: "ai-gateway",
				},
				Spec: runtimev1alpha1.AiGatewaySpec{
					AiModels: []runtimev1alpha1.AiModel{
						{
							Name:     "gpt-4",
							Provider: "openai",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, aiGateway)).To(Succeed())

			// Create agent with explicit AiGateway reference
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-with-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					AiGatewayRef: &corev1.ObjectReference{
						Name:      "test-reconcile-gateway",
						Namespace: "ai-gateway",
					},
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Reconcile the agent
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-agent-with-gateway",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify agent status has AiGatewayRef populated
			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-with-gateway", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.AiGatewayRef).NotTo(BeNil())
			Expect(updatedAgent.Status.AiGatewayRef.Name).To(Equal("test-reconcile-gateway"))
			Expect(updatedAgent.Status.AiGatewayRef.Namespace).To(Equal("ai-gateway"))

			// Verify deployment has LiteLLM environment variables
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-with-gateway", Namespace: "default"}, deployment)).To(Succeed())

			container := deployment.Spec.Template.Spec.Containers[0]
			proxyBaseVar := findEnvVar(container.Env, "LITELLM_PROXY_API_BASE")
			Expect(proxyBaseVar).NotTo(BeNil())
			Expect(proxyBaseVar.Value).To(Equal("http://test-reconcile-gateway.ai-gateway.svc.cluster.local.:4000"))

			// Clean up ai-gateway namespace resources
			Expect(k8sClient.Delete(ctx, aiGateway)).To(Succeed())
		})
	})

	Describe("ensureDeployment", func() {
		It("should create deployment with correct configuration", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:v1",
					Replicas:  func() *int32 { i := int32(2); return &i }(),
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/", Name: "a2a"},
					},
					Env: []corev1.EnvVar{
						{Name: "TEST_VAR", Value: "test-value"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-deployment", Namespace: "default"}, deployment)).To(Succeed())

			Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("agent"))
			Expect(container.Image).To(Equal("test-image:v1"))
			Expect(container.Ports).To(HaveLen(1))
			Expect(container.Ports[0].Name).To(Equal("a2a"))
			Expect(container.Ports[0].ContainerPort).To(Equal(int32(8000)))

			// Check user env var exists
			testVar := findEnvVar(container.Env, "TEST_VAR")
			Expect(testVar).NotTo(BeNil())
			Expect(testVar.Value).To(Equal("test-value"))
		})

		It("should update existing deployment", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-update-deployment",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:v1",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Create initial deployment
			err := reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			// Update agent image
			agent.Spec.Image = "test-image:v2"

			// Update deployment
			err = reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-update-deployment", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("test-image:v2"))
		})

		It("should add LiteLLM environment variables when AiGateway is connected", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway-env-vars",
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
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Create deployment without AiGateway
			err := reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-gateway-env-vars", Namespace: "default"}, deployment)).To(Succeed())

			// Verify LiteLLM vars are NOT present initially
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(findEnvVar(container.Env, "LITELLM_PROXY_API_BASE")).To(BeNil())

			// Now create an AiGateway and update deployment
			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "ai-gateway-ns",
				},
				Spec: runtimev1alpha1.AiGatewaySpec{
					Port: 4000,
				},
			}

			err = reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, aiGateway)
			Expect(err).NotTo(HaveOccurred())

			// Get updated deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-gateway-env-vars", Namespace: "default"}, deployment)).To(Succeed())
			container = deployment.Spec.Template.Spec.Containers[0]

			// Verify LiteLLM environment variables are now present
			proxyBaseVar := findEnvVar(container.Env, "LITELLM_PROXY_API_BASE")
			Expect(proxyBaseVar).NotTo(BeNil())
			Expect(proxyBaseVar.Value).To(Equal("http://test-gateway.ai-gateway-ns.svc.cluster.local.:4000"))

			proxyKeyVar := findEnvVar(container.Env, "LITELLM_PROXY_API_KEY")
			Expect(proxyKeyVar).NotTo(BeNil())
			Expect(proxyKeyVar.Value).To(Equal("NOT_USED_BY_GATEWAY"))

			useLiteLLMVar := findEnvVar(container.Env, "USE_LITELLM_PROXY")
			Expect(useLiteLLMVar).NotTo(BeNil())
			Expect(useLiteLLMVar.Value).To(Equal("True"))
		})

		It("should set readiness probe for A2A agents", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-probe",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/api"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-probe", Namespace: "default"}, deployment)).To(Succeed())

			probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal("/api" + agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(8000))
		})

		It("should merge user and template environment variables", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-env-merge",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework:   "google-adk",
					Image:       "test-image:latest",
					Description: "Test agent",
					Model:       "test-model",
					Env: []corev1.EnvVar{
						{Name: "USER_VAR", Value: "user-value"},
						{Name: "AGENT_MODEL", Value: "override-model"}, // Override template var
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.ensureDeployment(ctx, agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-env-merge", Namespace: "default"}, deployment)).To(Succeed())

			container := deployment.Spec.Template.Spec.Containers[0]

			// User var should exist
			userVar := findEnvVar(container.Env, "USER_VAR")
			Expect(userVar).NotTo(BeNil())
			Expect(userVar.Value).To(Equal("user-value"))

			// User should override template var
			modelVar := findEnvVar(container.Env, "AGENT_MODEL")
			Expect(modelVar).NotTo(BeNil())
			Expect(modelVar.Value).To(Equal("override-model"))

			// Template var should still exist
			descVar := findEnvVar(container.Env, "AGENT_DESCRIPTION")
			Expect(descVar).NotTo(BeNil())
			Expect(descVar.Value).To(Equal("Test agent"))
		})
	})

	Describe("ensureService", func() {
		It("should create service with correct ports", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/", Name: "a2a"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.ensureService(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-service", Namespace: "default"}, service)).To(Succeed())

			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Name).To(Equal("a2a"))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8000)))
			Expect(service.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(8000)))
		})

		It("should delete service when no protocols are defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-protocol",
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
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Create service
			err := reconciler.ensureService(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-no-protocol", Namespace: "default"}, service)).To(Succeed())

			// Remove protocols
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{}

			// Ensure service - should delete
			err = reconciler.ensureService(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-no-protocol", Namespace: "default"}, service)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should update service ports when protocols change", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-update-service",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/", Name: "a2a"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			// Create initial service
			err := reconciler.ensureService(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			// Update protocol port
			agent.Spec.Protocols[0].Port = 9000

			// Update service
			err = reconciler.ensureService(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-update-service", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(9000)))
		})
	})
})
