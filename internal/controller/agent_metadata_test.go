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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Metadata", func() {
	ctx := context.Background()
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: &events.FakeRecorder{},
		}
		Expect(os.Setenv("POD_NAMESPACE", "default")).To(Succeed())
	})

	AfterEach(func() {
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList)).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}
		_ = os.Unsetenv("POD_NAMESPACE")
	})

	Describe("CommonMetadata", func() {
		It("should apply commonMetadata labels and annotations to Deployment and Service", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-common-meta",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "image:tag",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
					CommonMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"team":        "platform",
							"environment": "test",
						},
						Annotations: map[string]string{
							"owner": "platform-team",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-agent-common-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify Deployment has commonMetadata labels and annotations
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-common-meta", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(deployment.Labels).To(HaveKeyWithValue("environment", "test"))
			Expect(deployment.Annotations).To(HaveKeyWithValue("owner", "platform-team"))

			// Verify Pod template has commonMetadata labels
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("environment", "test"))
			// Selector labels must still be present
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-agent-common-meta"))
			// Verify pod template annotations
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("owner", "platform-team"))

			// Verify Service has commonMetadata labels and annotations
			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-common-meta", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(service.Labels).To(HaveKeyWithValue("environment", "test"))
			Expect(service.Annotations).To(HaveKeyWithValue("owner", "platform-team"))
		})
	})

	Describe("PodMetadata", func() {
		It("should apply podMetadata labels and annotations only to the pod template", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-pod-meta",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "image:tag",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
					PodMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"pod-label": "pod-value",
						},
						Annotations: map[string]string{
							"pod-annotation": "pod-annotation-value",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-agent-pod-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-pod-meta", Namespace: "default"}, deployment)).To(Succeed())

			// Deployment itself should NOT have podMetadata labels
			Expect(deployment.Labels).NotTo(HaveKey("pod-label"))
			Expect(deployment.Annotations).To(BeNil())

			// Pod template should have podMetadata labels and annotations
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("pod-label", "pod-value"))
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("pod-annotation", "pod-annotation-value"))
			// Selector labels must still be present
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-agent-pod-meta"))

			// Service should NOT have podMetadata labels
			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-pod-meta", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Labels).NotTo(HaveKey("pod-label"))
		})
	})

	Describe("CommonMetadata and PodMetadata combined", func() {
		It("should apply both commonMetadata and podMetadata with podMetadata taking precedence on the pod template", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-both-meta",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "image:tag",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
					CommonMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"common-label": "common-value",
							"shared-label": "common-shared",
						},
						Annotations: map[string]string{
							"common-annotation": "common-value",
						},
					},
					PodMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"pod-only-label": "pod-value",
							"shared-label":   "pod-overridden",
						},
						Annotations: map[string]string{
							"pod-annotation": "pod-value",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-agent-both-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-both-meta", Namespace: "default"}, deployment)).To(Succeed())

			// Deployment should have commonMetadata labels but NOT podMetadata-only labels
			Expect(deployment.Labels).To(HaveKeyWithValue("common-label", "common-value"))
			Expect(deployment.Labels).NotTo(HaveKey("pod-only-label"))

			// Pod template should have both commonMetadata and podMetadata labels
			// podMetadata overrides commonMetadata for the same key
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("common-label", "common-value"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("pod-only-label", "pod-value"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("shared-label", "pod-overridden"))
			// Selector labels must still be present
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-agent-both-meta"))

			// Pod template annotations: both common and pod annotations present
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("common-annotation", "common-value"))
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("pod-annotation", "pod-value"))
		})
	})

	Describe("Selector labels are never overridden", func() {
		It("should not allow commonMetadata or podMetadata to override selector labels", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-selector-override",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "image:tag",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
					CommonMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"app": "user-override-attempt",
						},
					},
					PodMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"app": "pod-override-attempt",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-agent-selector-override", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-agent-selector-override", Namespace: "default"}, deployment)).To(Succeed())

			// Selector label "app" must still match the agent name
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-agent-selector-override"))
			Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", "test-agent-selector-override"))
		})
	})
})

var _ = Describe("ToolServer Metadata", func() {
	ctx := context.Background()
	var reconciler *ToolServerReconciler

	BeforeEach(func() {
		reconciler = &ToolServerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		toolServerList := &runtimev1alpha1.ToolServerList{}
		Expect(k8sClient.List(ctx, toolServerList)).To(Succeed())
		for i := range toolServerList.Items {
			_ = k8sClient.Delete(ctx, &toolServerList.Items[i])
		}
	})

	Describe("CommonMetadata", func() {
		It("should apply commonMetadata labels and annotations to Deployment and Service", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ts-common-meta",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "image:tag",
					Port:          8080,
					CommonMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"team":        "platform",
							"environment": "test",
						},
						Annotations: map[string]string{
							"owner": "platform-team",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-ts-common-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify Deployment has commonMetadata labels and annotations
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-ts-common-meta", Namespace: "default"}, deployment)).To(Succeed())
			Expect(deployment.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(deployment.Labels).To(HaveKeyWithValue("environment", "test"))
			Expect(deployment.Annotations).To(HaveKeyWithValue("owner", "platform-team"))

			// Verify pod template has commonMetadata labels and annotations
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("environment", "test"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-ts-common-meta"))
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("owner", "platform-team"))

			// Verify Service has commonMetadata labels and annotations
			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-ts-common-meta", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Labels).To(HaveKeyWithValue("team", "platform"))
			Expect(service.Labels).To(HaveKeyWithValue("environment", "test"))
			Expect(service.Annotations).To(HaveKeyWithValue("owner", "platform-team"))
		})
	})

	Describe("PodMetadata", func() {
		It("should apply podMetadata labels and annotations only to the pod template", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ts-pod-meta",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.ToolServerSpec{
					Protocol:      "mcp",
					TransportType: "http",
					Image:         "image:tag",
					Port:          8080,
					PodMetadata: &runtimev1alpha1.EmbeddedMetadata{
						Labels: map[string]string{
							"pod-label": "pod-value",
						},
						Annotations: map[string]string{
							"pod-annotation": "pod-annotation-value",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, toolServer)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-ts-pod-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-ts-pod-meta", Namespace: "default"}, deployment)).To(Succeed())

			// Deployment itself should NOT have podMetadata labels
			Expect(deployment.Labels).NotTo(HaveKey("pod-label"))
			Expect(deployment.Annotations).To(BeNil())

			// Pod template should have podMetadata labels and annotations
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("pod-label", "pod-value"))
			Expect(deployment.Spec.Template.Annotations).To(HaveKeyWithValue("pod-annotation", "pod-annotation-value"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-ts-pod-meta"))

			// Service should NOT have podMetadata labels
			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-ts-pod-meta", Namespace: "default"}, service)).To(Succeed())
			Expect(service.Labels).NotTo(HaveKey("pod-label"))
		})
	})

	Describe("No metadata", func() {
		It("should work without any metadata configured", func() {
			toolServer := &runtimev1alpha1.ToolServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ts-no-meta",
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
				NamespacedName: types.NamespacedName{Name: "test-ts-no-meta", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-ts-no-meta", Namespace: "default"}, deployment)).To(Succeed())

			// Pod template annotations should be nil when no metadata is specified
			Expect(deployment.Spec.Template.Annotations).To(BeNil())

			// Selector labels must be present
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "test-ts-no-meta"))

			// List resources to confirm cleanup
			_ = k8sClient.List(ctx, &runtimev1alpha1.ToolServerList{}, client.InNamespace("default"))
		})
	})
})
