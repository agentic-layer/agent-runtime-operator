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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolServer Controller", func() {
	Context("When reconciling an http transport ToolServer", func() {
		const resourceName = "test-http-toolserver"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for http ToolServer")
			toolserver := &runtimev1alpha1.ToolServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, toolserver)
			if err != nil && errors.IsNotFound(err) {
				replicas := int32(2)
				resource := &runtimev1alpha1.ToolServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: runtimev1alpha1.ToolServerSpec{
						Protocol:      "mcp",
						TransportType: "http",
						Image:         "python:3.11",
						Command:       []string{"python"},
						Args:          []string{"src/main.py", "--port", "8080"},
						Port:          8080,
						Path:          "/mcp",
						Replicas:      &replicas,
						Env: []corev1.EnvVar{
							{Name: "LOGLEVEL", Value: "INFO"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.ToolServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ToolServer")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create deployment and service for http transport", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ToolServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that deployment was created with correct spec")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, deploymentKey, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("toolserver"))
			Expect(container.Image).To(Equal("python:3.11"))
			Expect(container.Command).To(Equal([]string{"python"}))
			Expect(container.Args).To(Equal([]string{"src/main.py", "--port", "8080"}))
			Expect(container.Ports).To(HaveLen(1))
			Expect(container.Ports[0].ContainerPort).To(Equal(int32(8080)))
			Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "LOGLEVEL", Value: "INFO"}))
			Expect(container.ReadinessProbe).NotTo(BeNil())
			Expect(container.ReadinessProbe.TCPSocket).NotTo(BeNil())
			Expect(container.ReadinessProbe.TCPSocket.Port.IntValue()).To(Equal(8080))

			By("Checking that service was created with correct spec")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, serviceKey, service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(service.Spec.Ports[0].TargetPort.IntValue()).To(Equal(8080))
			Expect(service.Spec.Selector["app"]).To(Equal(resourceName))

			By("Verifying status is updated with URL immediately (optimistic)")
			toolserver := &runtimev1alpha1.ToolServer{}
			err = k8sClient.Get(ctx, typeNamespacedName, toolserver)
			Expect(err).NotTo(HaveOccurred())
			expectedURL := "http://test-http-toolserver.default.svc.cluster.local:8080/mcp"
			Expect(toolserver.Status.Url).To(Equal(expectedURL))
			Expect(toolserver.Status.Conditions).NotTo(BeEmpty())
			readyCondition := findCondition(toolserver.Status.Conditions, "Ready")
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should update deployment when spec changes", func() {
			By("Reconciling the created resource initially")
			controllerReconciler := &ToolServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ToolServer spec")
			toolserver := &runtimev1alpha1.ToolServer{}
			err = k8sClient.Get(ctx, typeNamespacedName, toolserver)
			Expect(err).NotTo(HaveOccurred())

			toolserver.Spec.Image = "python:3.12"
			newReplicas := int32(3)
			toolserver.Spec.Replicas = &newReplicas
			toolserver.Spec.Env = []corev1.EnvVar{
				{Name: "LOGLEVEL", Value: "DEBUG"},
			}
			err = k8sClient.Update(ctx, toolserver)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling after update")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying deployment was updated")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, deploymentKey, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("python:3.12"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
				corev1.EnvVar{Name: "LOGLEVEL", Value: "DEBUG"},
			))
		})
	})

	Context("When reconciling an sse transport ToolServer", func() {
		const resourceName = "test-sse-toolserver"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for sse ToolServer")
			toolserver := &runtimev1alpha1.ToolServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, toolserver)
			if err != nil && errors.IsNotFound(err) {
				replicas := int32(1)
				resource := &runtimev1alpha1.ToolServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: runtimev1alpha1.ToolServerSpec{
						Protocol:      "mcp",
						TransportType: "sse",
						Image:         "ghcr.io/example/sse-toolserver:v1.0.0",
						Port:          9090,
						Path:          "/sse",
						Replicas:      &replicas,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.ToolServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ToolServer")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create deployment and service for sse transport", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ToolServerReconciler{
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
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(9090)))

			By("Checking that service was created")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, serviceKey, service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(9090)))

			By("Verifying status URL for sse transport (optimistic)")
			toolserver := &runtimev1alpha1.ToolServer{}
			err = k8sClient.Get(ctx, typeNamespacedName, toolserver)
			Expect(err).NotTo(HaveOccurred())
			expectedURL := "http://test-sse-toolserver.default.svc.cluster.local:9090/sse"
			Expect(toolserver.Status.Url).To(Equal(expectedURL))
		})
	})
})

// Helper function to find a condition by type
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
