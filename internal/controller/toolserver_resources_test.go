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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolServer Resources", func() {
	ctx := context.Background()
	var reconciler *ToolServerReconciler

	BeforeEach(func() {
		reconciler = &ToolServerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		// Clean up all tool servers in the default namespace after each test
		toolServerList := &runtimev1alpha1.ToolServerList{}
		Expect(k8sClient.List(ctx, toolServerList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range toolServerList.Items {
			_ = k8sClient.Delete(ctx, &toolServerList.Items[i])
		}
	})

	Describe("Default Resource Configuration", func() {
		It("should apply default resource requests and limits when not specified", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-default-resources-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "image:tag",
					Port:          8080,
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-default-resources-toolserver",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was created with default resources
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-default-resources-toolserver", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]

			// Verify default memory requests and limits
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Requests[corev1.ResourceMemory].Equal(resource.MustParse("300Mi"))).To(BeTrue())
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Limits[corev1.ResourceMemory].Equal(resource.MustParse("500Mi"))).To(BeTrue())

			// Verify default CPU requests and limits
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Requests[corev1.ResourceCPU].Equal(resource.MustParse("100m"))).To(BeTrue())
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Limits[corev1.ResourceCPU].Equal(resource.MustParse("500m"))).To(BeTrue())
		})
	})

	Describe("Custom Resource Configuration", func() {
		It("should use custom resource requests and limits when specified", func() {
			customResources := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
					corev1.ResourceCPU:    resource.MustParse("250m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					corev1.ResourceCPU:    resource.MustParse("1000m"),
				},
			}

			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-custom-resources-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "image:tag",
					Port:          8080,
					Resources:     customResources,
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-custom-resources-toolserver",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was created with custom resources
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-custom-resources-toolserver", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]

			// Verify custom memory requests and limits
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Requests[corev1.ResourceMemory].Equal(resource.MustParse("512Mi"))).To(BeTrue())
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Limits[corev1.ResourceMemory].Equal(resource.MustParse("1Gi"))).To(BeTrue())

			// Verify custom CPU requests and limits
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Requests[corev1.ResourceCPU].Equal(resource.MustParse("250m"))).To(BeTrue())
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Limits[corev1.ResourceCPU].Equal(resource.MustParse("1000m"))).To(BeTrue())
		})

		It("should handle partial resource configuration", func() {
			partialResources := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			}

			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-partial-resources-toolserver",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "image:tag",
					Port:          8080,
					Resources:     partialResources,
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-partial-resources-toolserver",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was created with partial resources
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-partial-resources-toolserver", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]

			// Verify only specified resources are set
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Requests[corev1.ResourceMemory].Equal(resource.MustParse("256Mi"))).To(BeTrue())
			Expect(container.Resources.Requests).NotTo(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Limits).To(BeEmpty())
		})
	})
})
