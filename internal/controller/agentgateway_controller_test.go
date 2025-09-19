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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/agentgateway"
)

var _ = Describe("Agent Gateway Controller", func() {
	Context("ConfigMap creation", func() {
		const gatewayName = "test-configmap-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
		})

		It("should create ConfigMap with embedded KrakenD configuration", func() {
			By("Creating provider and resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			// Create resources using provider
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was created")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			By("Verifying ConfigMap structure")
			Expect(configMap.Name).To(Equal(configMapName))
			Expect(configMap.Namespace).To(Equal("default"))
			Expect(configMap.Data).To(HaveKey("krakend.json"))

			By("Verifying KrakenD configuration structure")
			assertKrakendConfigStructure(configMap.Data["krakend.json"])

			By("Verifying owner reference")
			Expect(configMap.OwnerReferences).To(HaveLen(1))
			Expect(configMap.OwnerReferences[0].Name).To(Equal(gatewayName))
		})
	})

	Context("Dynamic agent discovery", func() {
		const gatewayName = "test-discovery-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway
		var exposedAgent, hiddenAgent *runtimev1alpha1.Agent
		var exposedService, hiddenService *corev1.Service

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

			// Create exposed agent with service
			exposedAgent = createTestAgent(ctx, "exposed-agent", true, 8080)
			exposedService = createTestServiceForAgent(ctx, exposedAgent, 8080)

			// Create hidden agent (not exposed) with service
			hiddenAgent = createTestAgent(ctx, "hidden-agent", false, 8080)
			hiddenService = createTestServiceForAgent(ctx, hiddenAgent, 8080)

		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
			cleanupTestResource(ctx, exposedAgent)
			cleanupTestResource(ctx, hiddenAgent)
			cleanupTestResource(ctx, exposedService)
			cleanupTestResource(ctx, hiddenService)
		})

		It("should discover only exposed agents", func() {
			By("Getting exposed agents using provider")
			// Create a mock reconciler just to access getExposedAgents
			mockReconciler := &AgentGatewayReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			agents, err := mockReconciler.getExposedAgents(ctx)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying only exposed agent is returned")
			Expect(agents).To(HaveLen(1))
			Expect(agents[0].Name).To(Equal("exposed-agent"))
			Expect(agents[0].Spec.Exposed).To(BeTrue())
		})

		It("should generate service URL from owner reference", func() {
			// This functionality is now internal to the KrakenD provider
			Skip("Service URL generation is now internal to provider implementation")
		})

		It("should generate correct endpoint for agent", func() {
			// This functionality is now internal to the KrakenD provider
			Skip("Endpoint generation is now internal to provider implementation")
		})

		It("should create ConfigMap with endpoints for exposed agents", func() {
			By("Creating provider and resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			// Create resources using provider with exposed agent
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{exposedAgent})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was created")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			By("Parsing KrakenD configuration")
			var config map[string]interface{}
			Expect(json.Unmarshal([]byte(configMap.Data["krakend.json"]), &config)).To(Succeed())

			By("Verifying endpoints exist")
			endpoints, ok := config["endpoints"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(endpoints).To(HaveLen(1))

			By("Verifying endpoint structure in config")
			endpoint := endpoints[0].(map[string]interface{})
			Expect(endpoint["endpoint"]).To(Equal("/exposed-agent/{anyPath}/"))
			Expect(endpoint["method"]).To(Equal("POST"))
			Expect(endpoint["output_encoding"]).To(Equal("no-op"))

			backend := endpoint["backend"].([]interface{})[0].(map[string]interface{})
			hosts := backend["host"].([]interface{})
			Expect(hosts).To(HaveLen(1))
			Expect(hosts[0]).To(Equal("http://exposed-agent.default.svc.cluster.local:8080"))
		})
	})

	Context("Multiple agents scenario", func() {
		const gatewayName = "test-multi-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway
		var agent1, agent2, agent3 *runtimev1alpha1.Agent
		var service1, service2, service3 *corev1.Service

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

			// Create multiple exposed agents
			agent1 = createTestAgent(ctx, "weather-agent", true, 8080)
			service1 = createTestServiceForAgent(ctx, agent1, 8080)

			agent2 = createTestAgent(ctx, "news-agent", true, 9000)
			service2 = createTestServiceForAgent(ctx, agent2, 9000)

			agent3 = createTestAgent(ctx, "calendar-agent", true, 8000)
			service3 = createTestServiceForAgent(ctx, agent3, 8000)

		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
			cleanupTestResource(ctx, agent1)
			cleanupTestResource(ctx, agent2)
			cleanupTestResource(ctx, agent3)
			cleanupTestResource(ctx, service1)
			cleanupTestResource(ctx, service2)
			cleanupTestResource(ctx, service3)
		})

		It("should create ConfigMap with endpoints for all exposed agents", func() {
			By("Creating provider and resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			// Create resources using provider with all agents
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1, agent2, agent3})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was created")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			By("Parsing KrakenD configuration")
			var config map[string]interface{}
			Expect(json.Unmarshal([]byte(configMap.Data["krakend.json"]), &config)).To(Succeed())

			By("Verifying all agents have endpoints")
			endpoints, ok := config["endpoints"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(endpoints).To(HaveLen(3))

			By("Verifying each endpoint")
			endpointPaths := make([]string, len(endpoints))
			for i, endpoint := range endpoints {
				ep := endpoint.(map[string]interface{})
				endpointPaths[i] = ep["endpoint"].(string)
			}

			Expect(endpointPaths).To(ContainElements(
				"/weather-agent/{anyPath}/",
				"/news-agent/{anyPath}/",
				"/calendar-agent/{anyPath}/",
			))
		})
	})

	Context("Error handling", func() {
		const gatewayName = "test-error-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
		})

		It("should handle no exposed agents gracefully", func() {
			By("Creating provider and resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			// Create resources using provider
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was created")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			By("Verifying empty endpoints array")
			var config map[string]interface{}
			Expect(json.Unmarshal([]byte(configMap.Data["krakend.json"]), &config)).To(Succeed())

			endpoints, ok := config["endpoints"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(endpoints).To(BeEmpty())
		})

		It("should return error when agent service not found", func() {
			// This functionality is now internal to the KrakenD provider
			Skip("Service URL generation is now internal to provider implementation")
		})
	})

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

			By("Checking that service was created")
			service := &corev1.Service{}
			serviceKey := types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
			err = k8sClient.Get(ctx, serviceKey, service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(10000)))
		})
	})

	Context("ConfigMap update behavior", func() {
		const gatewayName = "test-update-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway
		var agent1, agent2 *runtimev1alpha1.Agent
		var service1, service2 *corev1.Service

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
					Timeout:  stringPtr("30000ms"),
					CacheTTL: stringPtr("120s"),
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

			// Create first agent
			agent1 = createTestAgent(ctx, "update-agent-1", true, 8080)
			service1 = createTestServiceForAgent(ctx, agent1, 8080)
		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
			cleanupTestResource(ctx, agent1)
			cleanupTestResource(ctx, service1)
			if agent2 != nil {
				cleanupTestResource(ctx, agent2)
			}
			if service2 != nil {
				cleanupTestResource(ctx, service2)
			}
		})

		It("should create initial ConfigMap with correct configuration", func() {
			By("Creating initial resources with one agent")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying initial ConfigMap was created")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			By("Verifying initial configuration content")
			var config map[string]interface{}
			Expect(json.Unmarshal([]byte(configMap.Data["krakend.json"]), &config)).To(Succeed())

			// Verify timeout and cacheTTL
			Expect(config["timeout"]).To(Equal("30000ms"))
			Expect(config["cache_ttl"]).To(Equal("120s"))

			// Verify single endpoint
			endpoints := config["endpoints"].([]interface{})
			Expect(endpoints).To(HaveLen(1))
			endpoint := endpoints[0].(map[string]interface{})
			Expect(endpoint["endpoint"]).To(Equal("/update-agent-1/{anyPath}/"))

			// Store initial resource version for comparison
			initialResourceVersion := configMap.ResourceVersion

			By("Adding a second agent and updating the configuration")
			agent2 = createTestAgent(ctx, "update-agent-2", true, 9000)
			service2 = createTestServiceForAgent(ctx, agent2, 9000)

			// Call provider again with both agents
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1, agent2})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was updated with new agent")
			updatedConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, updatedConfigMap)
				if err != nil {
					return false
				}
				// Check if resource version changed (indicating an update)
				return updatedConfigMap.ResourceVersion != initialResourceVersion
			}).Should(BeTrue())

			By("Verifying updated configuration contains both endpoints")
			var updatedConfig map[string]interface{}
			Expect(json.Unmarshal([]byte(updatedConfigMap.Data["krakend.json"]), &updatedConfig)).To(Succeed())

			updatedEndpoints := updatedConfig["endpoints"].([]interface{})
			Expect(updatedEndpoints).To(HaveLen(2))

			// Check that both agents are present
			endpointPaths := make([]string, 0, 2)
			for _, ep := range updatedEndpoints {
				endpoint := ep.(map[string]interface{})
				endpointPaths = append(endpointPaths, endpoint["endpoint"].(string))
			}
			Expect(endpointPaths).To(ContainElement("/update-agent-1/{anyPath}/"))
			Expect(endpointPaths).To(ContainElement("/update-agent-2/{anyPath}/"))
		})

		It("should update ConfigMap when AgentGateway spec changes", func() {
			By("Creating initial resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial ConfigMap")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			initialResourceVersion := configMap.ResourceVersion

			By("Updating AgentGateway timeout and cacheTTL")
			// Update the AgentGateway spec
			updatedAgentGateway := agentGateway.DeepCopy()
			updatedAgentGateway.Spec.Timeout = stringPtr("45000ms")
			updatedAgentGateway.Spec.CacheTTL = stringPtr("600s")

			// Call provider with updated AgentGateway
			err = provider.CreateAgentGatewayResources(ctx, updatedAgentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was updated with new timeout and cacheTTL")
			updatedConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, updatedConfigMap)
				if err != nil {
					return false
				}
				// Check if resource version changed
				return updatedConfigMap.ResourceVersion != initialResourceVersion
			}).Should(BeTrue())

			By("Verifying new configuration values")
			var updatedConfig map[string]interface{}
			Expect(json.Unmarshal([]byte(updatedConfigMap.Data["krakend.json"]), &updatedConfig)).To(Succeed())

			Expect(updatedConfig["timeout"]).To(Equal("45000ms"))
			Expect(updatedConfig["cache_ttl"]).To(Equal("600s"))
		})

		It("should not update ConfigMap when configuration hasn't changed", func() {
			By("Creating initial resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial ConfigMap")
			configMapName := agentGateway.Name + "-krakend-config"
			configMap := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, configMap)
			}).Should(Succeed())

			initialResourceVersion := configMap.ResourceVersion

			By("Calling provider again with same configuration")
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying ConfigMap was NOT updated (resource version unchanged)")
			unchangedConfigMap := &corev1.ConfigMap{}
			Consistently(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: agentGateway.Namespace}, unchangedConfigMap)
				Expect(err).NotTo(HaveOccurred())
				return unchangedConfigMap.ResourceVersion
			}, "2s", "200ms").Should(Equal(initialResourceVersion))
		})
	})

	Context("Deployment update behavior", func() {
		const gatewayName = "test-deployment-update-gateway"
		ctx := context.Background()

		var agentGateway *runtimev1alpha1.AgentGateway
		var agent1 *runtimev1alpha1.Agent
		var service1 *corev1.Service

		BeforeEach(func() {
			agentGateway = &runtimev1alpha1.AgentGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gatewayName,
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentGatewaySpec{
					Provider: "krakend",
					Replicas: int32Ptr(2),
				},
			}
			Expect(k8sClient.Create(ctx, agentGateway)).To(Succeed())

			agent1 = createTestAgent(ctx, "deploy-update-agent", true, 8080)
			service1 = createTestServiceForAgent(ctx, agent1, 8080)
		})

		AfterEach(func() {
			cleanupTestResource(ctx, agentGateway)
			cleanupTestResource(ctx, agent1)
			cleanupTestResource(ctx, service1)
		})

		It("should create initial Deployment with correct replica count", func() {
			By("Creating initial resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying initial Deployment was created")
			deploymentName := agentGateway.Name
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, deployment)
			}).Should(Succeed())

			By("Verifying initial configuration")
			Expect(deployment.Spec.Replicas).NotTo(BeNil())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
			Expect(deployment.Labels["provider"]).To(Equal("krakend"))

			initialGeneration := deployment.Generation

			By("Updating AgentGateway replica count")
			updatedAgentGateway := agentGateway.DeepCopy()
			updatedAgentGateway.Spec.Replicas = int32Ptr(4)

			err = provider.CreateAgentGatewayResources(ctx, updatedAgentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was updated with new replica count")
			updatedDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, updatedDeployment)
				if err != nil {
					return false
				}
				// Check if generation changed (indicating an update) and replica count is updated
				return updatedDeployment.Generation > initialGeneration &&
					updatedDeployment.Spec.Replicas != nil &&
					*updatedDeployment.Spec.Replicas == 4
			}).Should(BeTrue())
		})

		It("should not update Deployment when configuration hasn't changed", func() {
			By("Creating initial resources")
			provider, err := agentgateway.NewAgentGatewayProvider(runtimev1alpha1.KrakenDProvider, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Getting initial Deployment")
			deploymentName := agentGateway.Name
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, deployment)
			}).Should(Succeed())

			initialGeneration := deployment.Generation

			By("Calling provider again with same configuration")
			err = provider.CreateAgentGatewayResources(ctx, agentGateway, []*runtimev1alpha1.Agent{agent1})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying Deployment was NOT updated (generation unchanged)")
			unchangedDeployment := &appsv1.Deployment{}
			Consistently(func() int64 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: agentGateway.Namespace}, unchangedDeployment)
				Expect(err).NotTo(HaveOccurred())
				return unchangedDeployment.Generation
			}, "2s", "200ms").Should(Equal(initialGeneration))
		})
	})
})

// Helper functions for testing

// createTestAgent creates a test Agent resource
func createTestAgent(ctx context.Context, name string, exposed bool, port int32) *runtimev1alpha1.Agent {
	namespace := "default"
	agent := &runtimev1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: runtimev1alpha1.AgentSpec{
			Framework: "google-adk",
			Image:     "test-agent:latest",
			Exposed:   exposed,
			Protocols: []runtimev1alpha1.AgentProtocol{
				{
					Name: "http",
					Type: "A2A",
					Port: port,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, agent)).To(Succeed())
	return agent
}

// createTestServiceForAgent creates a test Service owned by an Agent
func createTestServiceForAgent(ctx context.Context, agent *runtimev1alpha1.Agent, port int32) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": agent.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Set Agent as the owner of the Service (simulating what the Agent controller does)
	Expect(ctrl.SetControllerReference(agent, service, k8sClient.Scheme())).To(Succeed())
	Expect(k8sClient.Create(ctx, service)).To(Succeed())
	return service
}

// cleanupTestResource deletes a test resource if it exists
func cleanupTestResource(ctx context.Context, obj client.Object) {
	err := k8sClient.Delete(ctx, obj)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

// assertKrakendConfigStructure verifies the KrakenD configuration structure
func assertKrakendConfigStructure(configData string) {
	var config map[string]interface{}
	Expect(json.Unmarshal([]byte(configData), &config)).To(Succeed())

	Expect(config).To(HaveKey("version"))
	Expect(config["version"]).To(Equal(float64(3)))
	Expect(config).To(HaveKey("port"))
	Expect(config["port"]).To(Equal(float64(8080)))
	Expect(config).To(HaveKey("name"))
	Expect(config["name"]).To(Equal("agent-gateway-krakend"))
	Expect(config).To(HaveKey("endpoints"))
}

// stringPtr returns a pointer to a string (helper for optional fields)
func stringPtr(s string) *string {
	return &s
}

// int32Ptr returns a pointer to an int32 (helper for optional fields)
func int32Ptr(i int32) *int32 {
	return &i
}
