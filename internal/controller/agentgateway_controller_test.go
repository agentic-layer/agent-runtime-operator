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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Gateway Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-gateway-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		agentGateway := &runtimev1alpha1.AgentGateway{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Agent Gateway")
			err := k8sClient.Get(ctx, typeNamespacedName, agentGateway)
			if err != nil && errors.IsNotFound(err) {
				resource := &runtimev1alpha1.AgentGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: runtimev1alpha1.AgentGatewaySpec{
						Provider: "krakend",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &runtimev1alpha1.AgentGateway{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Agent Gateway")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AgentGatewayReconciler{
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
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("eu.gcr.io/agentic-layer/agent-gateway-krakend:main"))

			//By("Checking that service was created")
			//service := &corev1.Service{}
			//serviceKey := types.NamespacedName{
			//	Name:      resourceName,
			//	Namespace: "default",
			//}
			//err = k8sClient.Get(ctx, serviceKey, service)
			//Expect(err).NotTo(HaveOccurred())
			//Expect(service.Spec.Ports).To(HaveLen(1))
			//Expect(service.Spec.Ports[0].Port).To(Equal(int32(8000)))
		})
	})
})
