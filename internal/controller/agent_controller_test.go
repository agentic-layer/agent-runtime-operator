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

	webhookv1alpha1 "github.com/agentic-layer/agent-runtime-operator/internal/webhook/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// resourceQuantity is a helper function to create resource quantities for tests
func resourceQuantity(s string) resource.Quantity {
	q, _ := resource.ParseQuantity(s)
	return q
}

var _ = Describe("Agent Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		agent := &runtimev1alpha1.Agent{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Agent")
			err := k8sClient.Get(ctx, typeNamespacedName, agent)
			if err != nil && errors.IsNotFound(err) {
				resource := &runtimev1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: runtimev1alpha1.AgentSpec{
						Framework: "google-adk",
						Image:     "ghcr.io/agentic-layer/weather-agent:0.3.0",
						Protocols: []runtimev1alpha1.AgentProtocol{
							{
								Type: "A2A",
								Port: 8000,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.Agent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Agent")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that deployment was created")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, deploymentKey, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/agentic-layer/weather-agent:0.3.0"))

			By("Checking that service was created")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, serviceKey, service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8000)))
		})
	})

	Context("When updating Agent resources", func() {
		const resourceName = "test-update-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the initial Agent resource")
			resource := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "ghcr.io/agentic-layer/weather-agent:0.3.0",
					Replicas:  func() *int32 { i := int32(1); return &i }(),
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "A2A",
							Port: 8000,
							Name: "a2a",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("Initial reconciliation to create resources")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.Agent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Agent")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should update Deployment when image changes", func() {
			By("Updating the Agent image")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Image = "ghcr.io/agentic-layer/weather-agent:0.3.0"
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Deployment was updated with new image")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			// Find agent container using our improved method
			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			Expect(agentContainer.Image).To(Equal("ghcr.io/agentic-layer/weather-agent:0.3.0"))
		})

		It("should detect environment variable changes and update deployment", func() {
			By("Verifying initial environment variables")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			// Now expect 7 template environment variables (6 original + A2A_AGENT_CARD_URL)
			Expect(agentContainer.Env).To(HaveLen(7))
			// Check AGENT_NAME env var
			agentNameVar := findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_update_resource"))

			By("Manually updating environment variables to simulate external changes")
			agentContainer.Env = append(agentContainer.Env, corev1.EnvVar{
				Name:  "NEW_VAR",
				Value: "test-value",
			})
			Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

			By("Reconciling should detect the difference and restore proper env vars")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying environment variables were restored to expected state")
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			agentContainer = findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			// Now expect 7 template environment variables (6 original + A2A_AGENT_CARD_URL)
			Expect(agentContainer.Env).To(HaveLen(7))
			// Check AGENT_NAME env var
			agentNameVar = findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_update_resource"))
		})

		It("should update Deployment when replicas change", func() {
			By("Updating the Agent replicas")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Replicas = func() *int32 { i := int32(3); return &i }()
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Deployment was updated with new replica count")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
		})

		It("should update Service when protocols change", func() {
			By("Updating the Agent protocols")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{
					Type: "A2A",
					Port: 9000,
					Name: "a2a-updated",
				},
				{
					Type: "OpenAI",
					Port: 9001,
					Name: "openai",
				},
			}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Service was updated with new ports")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			Expect(service.Spec.Ports).To(HaveLen(2))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(9000)))
			Expect(service.Spec.Ports[0].Name).To(Equal("a2a-updated"))
			Expect(service.Spec.Ports[1].Port).To(Equal(int32(9001)))
			Expect(service.Spec.Ports[1].Name).To(Equal("openai"))

			By("Verifying the Deployment container ports were updated")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(HaveLen(2))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(9000)))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].Name).To(Equal("a2a-updated"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort).To(Equal(int32(9001)))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[1].Name).To(Equal("openai"))
		})

		It("should delete Service when all protocols are removed", func() {
			By("Removing all protocols from Agent")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Service was deleted")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			err = k8sClient.Get(ctx, serviceKey, service)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("Verifying the Deployment still exists but with no container ports")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(BeEmpty())
		})

		It("should handle multiple simultaneous updates", func() {
			By("Updating image, replicas, and protocols simultaneously")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Image = "ghcr.io/agentic-layer/weather-agent:0.3.0"
			agent.Spec.Replicas = func() *int32 { i := int32(5); return &i }()
			agent.Spec.Framework = "custom"
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{
					Type: "OpenAI",
					Port: 8080,
					Name: "openai-api",
				},
			}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all Deployment updates were applied")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/agentic-layer/weather-agent:0.3.0"))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(5)))
			Expect(deployment.Labels["framework"]).To(Equal("custom"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8080)))

			By("Verifying all Service updates were applied")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(service.Spec.Ports[0].Name).To(Equal("openai-api"))
		})

		It("should preserve unmanaged fields in Deployment", func() {
			By("Manually adding resource limits to deployment")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			// Add resource limits that our controller doesn't manage
			deployment.Spec.Template.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU:    resourceQuantity("100m"),
				corev1.ResourceMemory: resourceQuantity("128Mi"),
			}
			Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

			By("Updating the Agent image")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())
			agent.Spec.Image = "ghcr.io/agentic-layer/weather-agent:0.4.0"
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the image was updated but resource limits were preserved")
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			Expect(agentContainer.Image).To(Equal("ghcr.io/agentic-layer/weather-agent:0.4.0"))
			Expect(agentContainer.Resources.Limits[corev1.ResourceCPU]).To(Equal(resourceQuantity("100m")))
			Expect(agentContainer.Resources.Limits[corev1.ResourceMemory]).To(Equal(resourceQuantity("128Mi")))
		})

		It("should update Deployment when env field is added", func() {
			By("Updating the Agent with a new env field")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			newEnvVar := corev1.EnvVar{
				Name:  "TEST_VAR",
				Value: "new-value",
			}
			agent.Spec.Env = []corev1.EnvVar{newEnvVar}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Deployment was updated with the new env var")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			// Check for the new env var and template env vars (6 original template + A2A_AGENT_CARD_URL + 1 user = 8 total)
			Expect(agentContainer.Env).To(HaveLen(8))
			Expect(agentContainer.Env).To(ContainElement(newEnvVar))
			Expect(agentContainer.Env).To(ContainElement(corev1.EnvVar{
				Name:  "AGENT_NAME",
				Value: "test_update_resource",
			}))
		})

		It("should update Deployment when envFrom field is added", func() {
			By("Updating the Agent with a new envFrom field")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			newEnvFromSource := corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "new-secret-ref",
					},
				},
			}
			agent.Spec.EnvFrom = []corev1.EnvFromSource{newEnvFromSource}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Deployment was updated with the new envFrom source")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			Expect(agentContainer.EnvFrom).To(HaveLen(1))
			Expect(agentContainer.EnvFrom).To(ContainElement(newEnvFromSource))
		})

		It("should fail reconciliation when an invalid envFrom source is added", func() {
			By("Creating prerequisite ConfigMap for this test only")
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "initial-config-for-invalid-test",
					Namespace: "default",
				},
				Data: map[string]string{"KEY": "VALUE"},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())
			// Use defer to ensure cleanup happens at the end of the test.
			defer func() {
				By("Cleaning up the prerequisite ConfigMap")
				Expect(k8sClient.Delete(ctx, configMap)).To(Succeed())
			}()

			By("Updating the Agent with an initial, valid envFrom source")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			validEnvFrom := corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			}
			agent.Spec.EnvFrom = []corev1.EnvFromSource{validEnvFrom}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the valid update")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Fetching the deployment to check its initial state")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			initialDeploymentEnvFrom := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers).EnvFrom

			By("Updating the Agent with an additional, invalid envFrom source")
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())
			invalidEnvFrom := corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "", // This name is invalid
					},
				},
			}
			agent.Spec.EnvFrom = append(agent.Spec.EnvFrom, invalidEnvFrom)
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling again, which should now fail")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			By("Verifying the Deployment's envFrom was not changed")
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			currentDeploymentEnvFrom := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers).EnvFrom
			Expect(currentDeploymentEnvFrom).To(Equal(initialDeploymentEnvFrom))
		})
	})

	Context("When reconciling template agents", func() {
		const resourceName = "test-template-agent"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a template Agent resource")
			resource := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework:   "google-adk",
					Image:       webhookv1alpha1.DefaultTemplateImageAdk,
					Description: "A test template agent",
					Instruction: "You are a helpful assistant",
					Model:       "gemini/gemini-2.5-flash",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "A2A",
							Port: 8000,
						},
					},
					SubAgents: []runtimev1alpha1.SubAgent{
						{
							Name: "test-sub-agent",
							Url:  "https://example.com/sub-agent.json",
						},
					},
					Tools: []runtimev1alpha1.AgentTool{
						{
							Name: "test-tool",
							Url:  "https://example.com/tool/mcp",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.Agent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the template Agent resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create deployment with all template environment variables", func() {
			By("Reconciling the template agent")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment was created with template environment variables")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			// Verify template environment variables
			agentNameVar := findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_template_agent"))

			agentDescVar := findEnvVar(agentContainer.Env, "AGENT_DESCRIPTION")
			Expect(agentDescVar).NotTo(BeNil())
			Expect(agentDescVar.Value).To(Equal("A test template agent"))

			agentInstVar := findEnvVar(agentContainer.Env, "AGENT_INSTRUCTION")
			Expect(agentInstVar).NotTo(BeNil())
			Expect(agentInstVar.Value).To(Equal("You are a helpful assistant"))

			agentModelVar := findEnvVar(agentContainer.Env, "AGENT_MODEL")
			Expect(agentModelVar).NotTo(BeNil())
			Expect(agentModelVar.Value).To(Equal("gemini/gemini-2.5-flash"))

			// Verify JSON-encoded fields
			subAgentsVar := findEnvVar(agentContainer.Env, "SUB_AGENTS")
			Expect(subAgentsVar).NotTo(BeNil())
			Expect(subAgentsVar.Value).To(ContainSubstring("test-sub-agent"))
			Expect(subAgentsVar.Value).To(ContainSubstring("https://example.com/sub-agent.json"))

			toolsVar := findEnvVar(agentContainer.Env, "AGENT_TOOLS")
			Expect(toolsVar).NotTo(BeNil())
			Expect(toolsVar.Value).To(ContainSubstring("test-tool"))
			Expect(toolsVar.Value).To(ContainSubstring("https://example.com/tool/mcp"))
		})

		It("should allow user environment variables to override template variables", func() {
			By("Updating the agent with user environment variables that override template vars")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			// Add user env vars that override template vars
			agent.Spec.Env = []corev1.EnvVar{
				{
					Name:  "AGENT_MODEL",
					Value: "user-override-model",
				},
				{
					Name:  "CUSTOM_VAR",
					Value: "custom-value",
				},
			}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying user environment variables override template variables")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			// Verify user override took precedence
			agentModelVar := findEnvVar(agentContainer.Env, "AGENT_MODEL")
			Expect(agentModelVar).NotTo(BeNil())
			Expect(agentModelVar.Value).To(Equal("user-override-model"))

			// Verify custom user var is present
			customVar := findEnvVar(agentContainer.Env, "CUSTOM_VAR")
			Expect(customVar).NotTo(BeNil())
			Expect(customVar.Value).To(Equal("custom-value"))

			// Verify template vars that weren't overridden are still present
			agentNameVar := findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_template_agent"))
		})

		It("should handle empty template fields gracefully", func() {
			By("Creating agent with empty template fields")
			emptyResourceName := "empty-template-agent"
			emptyTypeNamespacedName := types.NamespacedName{
				Name:      emptyResourceName,
				Namespace: "default",
			}

			resource := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      emptyResourceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     webhookv1alpha1.DefaultTemplateImageAdk,
					// All template fields are empty
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "A2A",
							Port: 8000,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			defer func() {
				By("Cleanup the empty template agent")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}()

			By("Reconciling the empty template agent")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: emptyTypeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment handles empty template fields gracefully")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: emptyResourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			// AGENT_NAME should always be set
			agentNameVar := findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("empty_template_agent"))

			// Empty fields should still have environment variables, but with empty values
			agentDescVar := findEnvVar(agentContainer.Env, "AGENT_DESCRIPTION")
			Expect(agentDescVar).NotTo(BeNil())
			Expect(agentDescVar.Value).To(Equal(""))

			// JSON fields with empty arrays should have empty JSON arrays
			subAgentsVar := findEnvVar(agentContainer.Env, "SUB_AGENTS")
			Expect(subAgentsVar).NotTo(BeNil())
			Expect(subAgentsVar.Value).To(Equal("{}"))

			toolsVar := findEnvVar(agentContainer.Env, "AGENT_TOOLS")
			Expect(toolsVar).NotTo(BeNil())
			Expect(toolsVar.Value).To(Equal("{}"))
		})

		It("should update deployment when template fields change", func() {
			By("Reconciling initial template agent")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Updating template fields")
			agent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, agent)).To(Succeed())

			agent.Spec.Description = "Updated description"
			agent.Spec.Model = "claude/claude-3.5-sonnet"
			agent.Spec.SubAgents = []runtimev1alpha1.SubAgent{
				{
					Name: "updated-sub-agent",
					Url:  "https://updated.com/sub-agent.json",
				},
				{
					Name: "second-sub-agent",
					Url:  "https://second.com/sub-agent.json",
				},
			}
			Expect(k8sClient.Update(ctx, agent)).To(Succeed())

			By("Reconciling the updated resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment was updated with new template values")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())

			// Verify updated template variables
			agentDescVar := findEnvVar(agentContainer.Env, "AGENT_DESCRIPTION")
			Expect(agentDescVar).NotTo(BeNil())
			Expect(agentDescVar.Value).To(Equal("Updated description"))

			agentModelVar := findEnvVar(agentContainer.Env, "AGENT_MODEL")
			Expect(agentModelVar).NotTo(BeNil())
			Expect(agentModelVar.Value).To(Equal("claude/claude-3.5-sonnet"))

			subAgentsVar := findEnvVar(agentContainer.Env, "SUB_AGENTS")
			Expect(subAgentsVar).NotTo(BeNil())
			Expect(subAgentsVar.Value).To(ContainSubstring("updated-sub-agent"))
			Expect(subAgentsVar.Value).To(ContainSubstring("second-sub-agent"))
		})
	})

	Context("When reconciling agents with custom images", func() {
		const resourceName = "test-custom-image"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating an Agent with custom image")
			resource := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "custom",
					Image:     "ghcr.io/custom/agent:1.0.0",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "OpenAI",
							Port: 8080,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.Agent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the custom image Agent resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create deployment with template environment variables even for custom images", func() {
			By("Reconciling the custom image agent")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment has template environment variables")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			Expect(agentContainer.Image).To(Equal("ghcr.io/custom/agent:1.0.0"))

			// Template env vars should be present even for custom images
			agentNameVar := findEnvVar(agentContainer.Env, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_custom_image"))

			// Other template vars should be present with empty values
			agentDescVar := findEnvVar(agentContainer.Env, "AGENT_DESCRIPTION")
			Expect(agentDescVar).NotTo(BeNil())

			subAgentsVar := findEnvVar(agentContainer.Env, "SUB_AGENTS")
			Expect(subAgentsVar).NotTo(BeNil())
			Expect(subAgentsVar.Value).To(Equal("{}"))
		})
	})

	Context("Unit tests for controller functions", func() {
		var reconciler *AgentReconciler

		BeforeEach(func() {
			reconciler = &AgentReconciler{}
		})

		Describe("buildTemplateEnvironmentVars", func() {
			It("should handle empty agent fields gracefully", func() {
				agent := &runtimev1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-agent",
					},
					Spec: runtimev1alpha1.AgentSpec{},
				}

				envVars, err := reconciler.buildTemplateEnvironmentVars(agent)
				Expect(err).NotTo(HaveOccurred())
				Expect(envVars).To(HaveLen(6))

				// Verify all required template variables are present with correct values
				agentNameVar := findEnvVar(envVars, "AGENT_NAME")
				Expect(agentNameVar).NotTo(BeNil())
				Expect(agentNameVar.Value).To(Equal("test_agent"))

				agentDescVar := findEnvVar(envVars, "AGENT_DESCRIPTION")
				Expect(agentDescVar).NotTo(BeNil())
				Expect(agentDescVar.Value).To(Equal(""))

				agentInstVar := findEnvVar(envVars, "AGENT_INSTRUCTION")
				Expect(agentInstVar).NotTo(BeNil())
				Expect(agentInstVar.Value).To(Equal(""))

				agentModelVar := findEnvVar(envVars, "AGENT_MODEL")
				Expect(agentModelVar).NotTo(BeNil())
				Expect(agentModelVar.Value).To(Equal(""))

				subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
				Expect(subAgentsVar).NotTo(BeNil())
				Expect(subAgentsVar.Value).To(Equal("{}"))

				toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
				Expect(toolsVar).NotTo(BeNil())
				Expect(toolsVar.Value).To(Equal("{}"))
			})

			It("should populate all template fields correctly", func() {
				agent := &runtimev1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full-agent",
					},
					Spec: runtimev1alpha1.AgentSpec{
						Description: "Test agent description",
						Instruction: "You are a helpful assistant",
						Model:       "gpt-4",
						SubAgents: []runtimev1alpha1.SubAgent{
							{Name: "sub1", Url: "https://example.com/sub1.json"},
							{Name: "sub2", Url: "https://example.com/sub2.json"},
						},
						Tools: []runtimev1alpha1.AgentTool{
							{Name: "tool1", Url: "https://example.com/tool1"},
							{Name: "tool2", Url: "https://example.com/tool2"},
						},
					},
				}

				envVars, err := reconciler.buildTemplateEnvironmentVars(agent)
				Expect(err).NotTo(HaveOccurred())
				Expect(envVars).To(HaveLen(6))

				agentDescVar := findEnvVar(envVars, "AGENT_DESCRIPTION")
				Expect(agentDescVar.Value).To(Equal("Test agent description"))

				agentInstVar := findEnvVar(envVars, "AGENT_INSTRUCTION")
				Expect(agentInstVar.Value).To(Equal("You are a helpful assistant"))

				agentModelVar := findEnvVar(envVars, "AGENT_MODEL")
				Expect(agentModelVar.Value).To(Equal("gpt-4"))

				subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
				Expect(subAgentsVar.Value).To(ContainSubstring("sub1"))
				Expect(subAgentsVar.Value).To(ContainSubstring("https://example.com/sub1.json"))
				Expect(subAgentsVar.Value).To(ContainSubstring("sub2"))

				toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
				Expect(toolsVar.Value).To(ContainSubstring("tool1"))
				Expect(toolsVar.Value).To(ContainSubstring("https://example.com/tool1"))
				Expect(toolsVar.Value).To(ContainSubstring("tool2"))
			})

			It("should handle JSON marshaling of complex structures", func() {
				agent := &runtimev1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "json-test-agent",
					},
					Spec: runtimev1alpha1.AgentSpec{
						SubAgents: []runtimev1alpha1.SubAgent{
							{Name: "test-sub", Url: "https://example.com/sub.json"},
						},
						Tools: []runtimev1alpha1.AgentTool{
							{Name: "test-tool", Url: "https://example.com/tool"},
						},
					},
				}

				envVars, err := reconciler.buildTemplateEnvironmentVars(agent)
				Expect(err).NotTo(HaveOccurred())

				// Verify JSON structure is valid
				subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
				Expect(subAgentsVar.Value).To(MatchJSON(`{"test-sub":{"url":"https://example.com/sub.json"}}`))

				toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
				Expect(toolsVar.Value).To(MatchJSON(`{"test-tool":{"url":"https://example.com/tool"}}`))
			})
		})

		Describe("mergeEnvironmentVariables", func() {
			It("should merge template and user variables with user precedence", func() {
				templateVars := []corev1.EnvVar{
					{Name: "AGENT_NAME", Value: "test_agent"},
					{Name: "AGENT_MODEL", Value: "default-model"},
					{Name: "TEMPLATE_ONLY", Value: "template-value"},
				}

				userVars := []corev1.EnvVar{
					{Name: "AGENT_MODEL", Value: "user-model"}, // Override
					{Name: "USER_ONLY", Value: "user-value"},   // New variable
				}

				result := reconciler.mergeEnvironmentVariables(templateVars, userVars)

				// Should have all unique variables
				Expect(result).To(HaveLen(4))

				// User variable should override template variable
				agentModelVar := findEnvVar(result, "AGENT_MODEL")
				Expect(agentModelVar).NotTo(BeNil())
				Expect(agentModelVar.Value).To(Equal("user-model"))

				// Template-only variable should remain
				templateOnlyVar := findEnvVar(result, "TEMPLATE_ONLY")
				Expect(templateOnlyVar).NotTo(BeNil())
				Expect(templateOnlyVar.Value).To(Equal("template-value"))

				// User-only variable should be present
				userOnlyVar := findEnvVar(result, "USER_ONLY")
				Expect(userOnlyVar).NotTo(BeNil())
				Expect(userOnlyVar.Value).To(Equal("user-value"))

				// Non-overridden template variable should remain
				agentNameVar := findEnvVar(result, "AGENT_NAME")
				Expect(agentNameVar).NotTo(BeNil())
				Expect(agentNameVar.Value).To(Equal("test_agent"))
			})

			It("should handle empty input slices", func() {
				// Empty template vars
				result := reconciler.mergeEnvironmentVariables([]corev1.EnvVar{}, []corev1.EnvVar{
					{Name: "USER_VAR", Value: "value"},
				})
				Expect(result).To(HaveLen(1))
				Expect(result[0].Name).To(Equal("USER_VAR"))

				// Empty user vars
				result = reconciler.mergeEnvironmentVariables([]corev1.EnvVar{
					{Name: "TEMPLATE_VAR", Value: "value"},
				}, []corev1.EnvVar{})
				Expect(result).To(HaveLen(1))
				Expect(result[0].Name).To(Equal("TEMPLATE_VAR"))

				// Both empty
				result = reconciler.mergeEnvironmentVariables([]corev1.EnvVar{}, []corev1.EnvVar{})
				Expect(result).To(BeEmpty())
			})

			It("should preserve environment variable ordering", func() {
				templateVars := []corev1.EnvVar{
					{Name: "A", Value: "template-a"},
					{Name: "B", Value: "template-b"},
					{Name: "C", Value: "template-c"},
				}

				userVars := []corev1.EnvVar{
					{Name: "B", Value: "user-b"}, // Override middle variable
				}

				result := reconciler.mergeEnvironmentVariables(templateVars, userVars)

				// Should maintain template variable order for non-overridden vars
				Expect(result[0].Name).To(Equal("A"))
				Expect(result[1].Name).To(Equal("B"))
				Expect(result[1].Value).To(Equal("user-b")) // But with user value
				Expect(result[2].Name).To(Equal("C"))
			})
		})
	})

	Context("When testing A2A_AGENT_CARD_URL generation", func() {
		var reconciler *AgentReconciler

		BeforeEach(func() {
			reconciler = &AgentReconciler{}
		})

		It("should generate correct A2A_AGENT_CARD_URL when A2A protocol is present", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-namespace",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "A2A",
							Port: 8080,
							Name: "a2a",
							Path: "/.well-known/agent-card.json",
						},
					},
				},
			}

			templateVars, err := reconciler.buildTemplateEnvironmentVars(agent)
			Expect(err).NotTo(HaveOccurred())

			// Find the A2A_AGENT_CARD_URL variable
			a2aUrlVar := findEnvVar(templateVars, "A2A_AGENT_CARD_URL")
			Expect(a2aUrlVar).NotTo(BeNil())
			Expect(a2aUrlVar.Value).To(Equal("http://test-agent.test-namespace.svc.cluster.local:8080/.well-known/agent-card.json"))
		})

		It("should not generate A2A_AGENT_CARD_URL when no A2A protocol is present", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-namespace",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "OpenAI",
							Port: 8000,
							Name: "openai",
							Path: "/v1/chat/completions",
						},
					},
				},
			}

			templateVars, err := reconciler.buildTemplateEnvironmentVars(agent)
			Expect(err).NotTo(HaveOccurred())

			// A2A_AGENT_CARD_URL should not be present
			a2aUrlVar := findEnvVar(templateVars, "A2A_AGENT_CARD_URL")
			Expect(a2aUrlVar).To(BeNil())
		})

		It("should not generate A2A_AGENT_CARD_URL when no protocols are defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-namespace",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{},
				},
			}

			templateVars, err := reconciler.buildTemplateEnvironmentVars(agent)
			Expect(err).NotTo(HaveOccurred())

			// A2A_AGENT_CARD_URL should not be present
			a2aUrlVar := findEnvVar(templateVars, "A2A_AGENT_CARD_URL")
			Expect(a2aUrlVar).To(BeNil())
		})

		It("should use the first A2A protocol when multiple A2A protocols are defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-namespace",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: "A2A",
							Port: 8080,
							Name: "a2a-first",
							Path: "/a2a",
						},
						{
							Type: "A2A",
							Port: 9090,
							Name: "a2a-second",
							Path: "/",
						},
					},
				},
			}

			templateVars, err := reconciler.buildTemplateEnvironmentVars(agent)
			Expect(err).NotTo(HaveOccurred())

			// Should use the first A2A protocol
			a2aUrlVar := findEnvVar(templateVars, "A2A_AGENT_CARD_URL")
			Expect(a2aUrlVar).NotTo(BeNil())
			Expect(a2aUrlVar.Value).To(Equal("http://test-agent.test-namespace.svc.cluster.local:8080/a2a"))
		})
	})
})

// Helper functions for tests
func findAgentContainerHelper(containers []corev1.Container) *corev1.Container {
	for i := range containers {
		if containers[i].Name == "agent" {
			return &containers[i]
		}
	}
	// Fallback to first container for backwards compatibility
	if len(containers) > 0 {
		return &containers[0]
	}
	return nil
}

// findEnvVar finds an environment variable by name in a slice of EnvVars
func findEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			return &envVars[i]
		}
	}
	return nil
}
