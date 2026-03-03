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
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Status", func() {
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
		// Clean up all agents in the default namespace after each test
		agentList := &runtimev1alpha1.AgentList{}
		Expect(k8sClient.List(ctx, agentList, &client.ListOptions{Namespace: "default"})).To(Succeed())
		for i := range agentList.Items {
			_ = k8sClient.Delete(ctx, &agentList.Items[i])
		}
	})

	Describe("updateAgentStatusReady", func() {
		It("should set Status.Url from A2A protocol", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-status-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: ""},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusReady(ctx, agent, nil)
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-status-url", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.Url).To(Equal("http://agent-status-url.default.svc.cluster.local:8000/.well-known/agent-card.json"))
		})

		It("should set Ready condition to True", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-condition-ready",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusReady(ctx, agent, nil)
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-condition-ready", Namespace: "default"}, updatedAgent)).To(Succeed())

			var condition *metav1.Condition
			for i := range updatedAgent.Status.Conditions {
				if updatedAgent.Status.Conditions[i].Type == "Ready" {
					condition = &updatedAgent.Status.Conditions[i]
					break
				}
			}

			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("Reconciled"))
			Expect(condition.Message).To(Equal("Agent is ready"))
		})

		It("should set empty URL when no A2A protocol is defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-no-protocol",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusReady(ctx, agent, nil)
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-no-protocol", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.Url).To(Equal(""))
		})

		It("should set Status.AiGatewayRef when AiGateway is provided", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: "ai-gateway",
				},
				Spec: runtimev1alpha1.AiGatewaySpec{
					Port: 4000,
				},
			}

			err := reconciler.updateAgentStatusReady(ctx, agent, aiGateway)
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-with-gateway", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.AiGatewayRef).NotTo(BeNil())
			Expect(updatedAgent.Status.AiGatewayRef.Name).To(Equal("test-gateway"))
			Expect(updatedAgent.Status.AiGatewayRef.Namespace).To(Equal("ai-gateway"))
		})

		It("should set AiGatewayRef to nil when no AiGateway is provided", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-without-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusReady(ctx, agent, nil)
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-without-gateway", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.AiGatewayRef).To(BeNil())
		})
	})

	Describe("updateAgentStatusNotReady", func() {
		It("should set Ready condition to False with custom reason", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-not-ready",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusNotReady(ctx, agent, "MissingSubAgents", "SubAgent 'test-sub' not found")
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-not-ready", Namespace: "default"}, updatedAgent)).To(Succeed())

			var condition *metav1.Condition
			for i := range updatedAgent.Status.Conditions {
				if updatedAgent.Status.Conditions[i].Type == "Ready" {
					condition = &updatedAgent.Status.Conditions[i]
					break
				}
			}

			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal("MissingSubAgents"))
			Expect(condition.Message).To(Equal("SubAgent 'test-sub' not found"))
		})

		It("should still update URL even when not ready", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-not-ready-url",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: ""},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusNotReady(ctx, agent, "TestReason", "Test message")
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-not-ready-url", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.Url).To(Equal(""))
		})

		It("should clear Status.AiGatewayRef when agent is not ready", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-not-ready-gateway",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusNotReady(ctx, agent, "TestReason", "Test message")
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-not-ready-gateway", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.AiGatewayRef).To(BeNil())
		})
	})

	Describe("getDeploymentReadiness", func() {
		It("should return ready=true when Available condition is True", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
					},
				},
			}
			ready, reason, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeTrue())
			Expect(reason).To(BeEmpty())
			Expect(message).To(BeEmpty())
		})

		It("should return ready=false when Available condition is False", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse},
					},
				},
			}
			ready, reason, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeFalse())
			Expect(reason).To(Equal("DeploymentNotReady"))
			Expect(message).To(ContainSubstring("Insufficient replicas available"))
		})

		It("should return ready=false with 'Referenced resources missing' when ReplicaFailure is True", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate"},
					},
				},
			}
			ready, reason, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeFalse())
			Expect(reason).To(Equal("DeploymentNotReady"))
			Expect(message).To(ContainSubstring("Referenced resources missing"))
		})

		It("should return ready=false with 'failed to progress' when ProgressDeadlineExceeded", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"},
					},
				},
			}
			ready, reason, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeFalse())
			Expect(reason).To(Equal("DeploymentNotReady"))
			Expect(message).To(ContainSubstring("failed to progress"))
		})

		It("should return ready=false when no conditions are set yet", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{},
			}
			ready, reason, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeFalse())
			Expect(reason).To(Equal("DeploymentNotReady"))
			Expect(message).To(ContainSubstring("Waiting for deployment to become available"))
		})

		It("should prioritize ReplicaFailure over other conditions", func() {
			deployment := &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse},
						{Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate"},
					},
				},
			}
			ready, _, message := getDeploymentReadiness(deployment)
			Expect(ready).To(BeFalse())
			Expect(message).To(ContainSubstring("Referenced resources missing"))
		})
	})

	Describe("updateAgentStatusDeploymentNotReady", func() {
		It("should set Ready condition to False while keeping URL and AiGatewayRef", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-deployment-not-ready",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			aiGateway := &runtimev1alpha1.AiGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway-deploy",
					Namespace: "ai-gateway",
				},
				Spec: runtimev1alpha1.AiGatewaySpec{Port: 4000},
			}

			err := reconciler.updateAgentStatusDeploymentNotReady(ctx, agent, aiGateway, "DeploymentNotReady", "Deployment not ready: Referenced resources missing")
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-deployment-not-ready", Namespace: "default"}, updatedAgent)).To(Succeed())

			// URL should still be populated
			Expect(updatedAgent.Status.Url).To(Equal("http://agent-deployment-not-ready.default.svc.cluster.local:8000/.well-known/agent-card.json"))

			// AiGatewayRef should still be populated
			Expect(updatedAgent.Status.AiGatewayRef).NotTo(BeNil())
			Expect(updatedAgent.Status.AiGatewayRef.Name).To(Equal("test-gateway-deploy"))

			// Ready condition should be False
			var condition *metav1.Condition
			for i := range updatedAgent.Status.Conditions {
				if updatedAgent.Status.Conditions[i].Type == "Ready" {
					condition = &updatedAgent.Status.Conditions[i]
					break
				}
			}
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Reason).To(Equal("DeploymentNotReady"))
			Expect(condition.Message).To(ContainSubstring("Referenced resources missing"))
		})

		It("should set AiGatewayRef to nil when no AiGateway is provided", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-deployment-not-ready-no-gw",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "google-adk",
					Image:     "test-image:latest",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).To(Succeed())

			err := reconciler.updateAgentStatusDeploymentNotReady(ctx, agent, nil, "DeploymentNotReady", "Deployment not ready: Waiting for deployment to become available")
			Expect(err).NotTo(HaveOccurred())

			updatedAgent := &runtimev1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "agent-deployment-not-ready-no-gw", Namespace: "default"}, updatedAgent)).To(Succeed())
			Expect(updatedAgent.Status.AiGatewayRef).To(BeNil())
		})
	})
})
