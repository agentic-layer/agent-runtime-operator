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
						Image:     "eu.gcr.io/agentic-layer/weather-agent:0.1.2",
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
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("eu.gcr.io/agentic-layer/weather-agent:0.1.2"))

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
					Image:     "eu.gcr.io/agentic-layer/weather-agent:0.1.0",
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

			agent.Spec.Image = "eu.gcr.io/agentic-layer/weather-agent:0.2.0"
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
			// Find agent container using our improved method (addresses PR feedback)
			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			Expect(agentContainer.Image).To(Equal("eu.gcr.io/agentic-layer/weather-agent:0.2.0"))
		})

		It("should detect environment variable changes and update deployment", func() {
			By("Verifying initial environment variables")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: resourceName, Namespace: "default"}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())

			agentContainer := findAgentContainerHelper(deployment.Spec.Template.Spec.Containers)
			Expect(agentContainer).NotTo(BeNil())
			Expect(agentContainer.Env).To(HaveLen(1))
			Expect(agentContainer.Env[0].Name).To(Equal("AGENT_NAME"))
			Expect(agentContainer.Env[0].Value).To(Equal(resourceName))

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
			Expect(agentContainer.Env).To(HaveLen(1))
			Expect(agentContainer.Env[0].Name).To(Equal("AGENT_NAME"))
			Expect(agentContainer.Env[0].Value).To(Equal(resourceName))
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

			agent.Spec.Image = "eu.gcr.io/agentic-layer/weather-agent:0.3.0"
			agent.Spec.Replicas = func() *int32 { i := int32(5); return &i }()
			agent.Spec.Framework = "flokk"
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
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("eu.gcr.io/agentic-layer/weather-agent:0.3.0"))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(5)))
			Expect(deployment.Labels["framework"]).To(Equal("flokk"))
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
			agent.Spec.Image = "eu.gcr.io/agentic-layer/weather-agent:0.4.0"
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
			Expect(agentContainer.Image).To(Equal("eu.gcr.io/agentic-layer/weather-agent:0.4.0"))
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

			// Check for both the default and the new env var
			Expect(agentContainer.Env).To(HaveLen(2))
			Expect(agentContainer.Env).To(ContainElement(newEnvVar))
			Expect(agentContainer.Env).To(ContainElement(corev1.EnvVar{
				Name:  "AGENT_NAME",
				Value: resourceName,
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
})

// Helper functions for tests (addresses PR feedback)
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
