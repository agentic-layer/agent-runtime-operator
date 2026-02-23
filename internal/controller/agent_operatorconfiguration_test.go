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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Controller - OperatorConfiguration", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		// Clean up all agents across all namespaces
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList)).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}

		// Clean up all operator configurations
		configList := &runtimev1alpha1.OperatorConfigurationList{}
		Expect(k8sClient.List(ctx, configList)).To(Succeed())
		for i := range configList.Items {
			_ = k8sClient.Delete(ctx, &configList.Items[i])
		}
	})

	Describe("OperatorConfiguration support", func() {
		It("should use built-in default image when no OperatorConfiguration exists", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-no-config",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					// No Image specified - should use default
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-agent-no-config",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment uses built-in default
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-agent-no-config",
				Namespace: "default",
			}, deployment)).To(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(defaultTemplateImageAdk))
		})

		It("should use custom image from OperatorConfiguration", func() {
			// Create OperatorConfiguration with custom image
			customImage := "custom-registry.io/agent-template-adk:1.0.0"
			config := &runtimev1alpha1.OperatorConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: runtimev1alpha1.OperatorConfigurationSpec{
					AgentTemplateImages: &runtimev1alpha1.AgentTemplateImages{
						GoogleAdk: customImage,
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-with-config",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					// No Image specified - should use config image
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-agent-with-config",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment uses config image
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-agent-with-config",
				Namespace: "default",
			}, deployment)).To(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(customImage))
		})

		It("should prioritize Agent.Spec.Image over OperatorConfiguration", func() {
			// Create OperatorConfiguration with custom image
			configImage := "config-registry.io/agent-template-adk:1.0.0"
			config := &runtimev1alpha1.OperatorConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config-priority",
				},
				Spec: runtimev1alpha1.OperatorConfigurationSpec{
					AgentTemplateImages: &runtimev1alpha1.AgentTemplateImages{
						GoogleAdk: configImage,
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			// Create agent with explicit image
			agentImage := "agent-specific.io/custom-image:2.0.0"
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-priority",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     agentImage, // Explicit image should take priority
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-agent-priority",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment uses agent-specific image
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-agent-priority",
				Namespace: "default",
			}, deployment)).To(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(agentImage))
		})

		It("should fall back to built-in default when OperatorConfiguration has empty image", func() {
			// Create OperatorConfiguration with empty image
			config := &runtimev1alpha1.OperatorConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config-empty",
				},
				Spec: runtimev1alpha1.OperatorConfigurationSpec{
					AgentTemplateImages: &runtimev1alpha1.AgentTemplateImages{
						GoogleAdk: "", // Empty string
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-empty-config",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-agent-empty-config",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment uses built-in default
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-agent-empty-config",
				Namespace: "default",
			}, deployment)).To(Succeed())

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(defaultTemplateImageAdk))
		})
	})
})
