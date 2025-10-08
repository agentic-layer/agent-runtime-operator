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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	webhookv1alpha1 "github.com/agentic-layer/agent-runtime-operator/internal/webhook/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "agent-runtime-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "agent-runtime-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "agent-runtime-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "agent-runtime-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		sampleImages := []string{
			"ghcr.io/agentic-layer/weather-agent:0.3.0",
			webhookv1alpha1.DefaultTemplateImageAdk,
		}

		By("loading the sample images on Kind")
		for _, img := range sampleImages {
			cmd = exec.Command("docker", "pull", img)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to pull image ", img)
			err = utils.LoadImageToKindClusterWithName(img)
			ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load image ", img, " into Kind")
		}
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=agent-runtime-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		It("should provisioned cert-manager", func() {
			By("validating that cert-manager has the certificate Secret")
			verifyCertManager := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyCertManager).Should(Succeed())
		})

		It("should have CA injection for mutating webhooks", func() {
			By("checking CA injection for mutating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"agent-runtime-operator-mutating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				mwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		It("should have CA injection for validating webhooks", func() {
			By("checking CA injection for validating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"validatingwebhookconfigurations.admissionregistration.k8s.io",
					"agent-runtime-operator-validating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				vwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(vwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	Context("Sample Agent Deployment", func() {
		const testNamespace = "default"
		const weatherAgentName = "weather-agent"
		const newsAgentName = "news-agent"
		const configMapName = "agent-config-map"

		BeforeAll(func() {
			By("waiting for webhook service to be ready")
			Eventually(func(g Gomega) {
				// Check that the webhook service exists and has endpoints
				cmd := exec.Command("kubectl", "get", "service",
					"agent-runtime-operator-webhook-service", "-n", "agent-runtime-operator-system")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Check that the webhook service has endpoints (meaning pods are ready)
				cmd = exec.Command("kubectl", "get", "endpoints", "agent-runtime-operator-webhook-service",
					"-n", "agent-runtime-operator-system", "-o", "jsonpath={.subsets[*].addresses[*].ip}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "Webhook service should have endpoints")
			}, 2*time.Minute, 5*time.Second).Should(Succeed(), "Webhook service should be ready")

			By("applying sample configmap")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/configmap.yaml",
				"-n", testNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply sample configmap")
		})

		AfterEach(func() {
			By("cleaning up test agents from individual tests")
			// Clean up test agents created in individual tests
			testAgents := []string{"test-subagent", "test-parent-agent", "watch-subagent", "watch-parent"}
			for _, agentName := range testAgents {
				cmd := exec.Command("kubectl", "delete", "agent", agentName, "-n", testNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)
			}
		})

		AfterAll(func() {
			By("cleaning up sample agents")
			cmd := exec.Command("kubectl", "delete", "-f", "config/samples/runtime_v1alpha1_agent.yaml",
				"-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "delete", "-f",
				"config/samples/runtime_v1alpha1_agent_template.yaml", "-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			By("cleaning up sample configmap")
			cmd = exec.Command("kubectl", "delete", "configmap", configMapName, "-n", testNamespace,
				"--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})

		It("should successfully deploy and manage the weather agent (custom image)", func() {
			By("applying the weather agent sample")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/runtime_v1alpha1_agent.yaml", "-n", testNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply weather agent sample")

			By("verifying the weather agent resource is created")
			verifyAgentExists := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", weatherAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyAgentExists).Should(Succeed())

			By("waiting for the weather agent deployment to be created and ready")
			verifyDeploymentReady := func(g Gomega) {
				// Check deployment exists
				cmd := exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Check deployment is ready
				cmd = exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Deployment should have 1 ready replica")

				// Check deployment status
				cmd = exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Deployment should be available")
			}
			Eventually(verifyDeploymentReady, 3*time.Minute).Should(Succeed())

			By("verifying the weather agent service is created")
			verifyServiceExists := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", weatherAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Verify service has the expected port
				cmd = exec.Command("kubectl", "get", "service", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.ports[0].port}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("8000"), "Service should expose port 8000")
			}
			Eventually(verifyServiceExists).Should(Succeed())

			By("verifying the weather agent pod is healthy")
			verifyPodHealthy := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "-l", "app="+weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.items[0].status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Weather agent pod should be running")

				// Check pod is ready
				cmd = exec.Command("kubectl", "get", "pods", "-l", "app="+weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.items[0].status.containerStatuses[0].ready}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "Weather agent pod should be ready")
			}
			Eventually(verifyPodHealthy, 2*time.Minute).Should(Succeed())

			By("verifying deployment has correct environment variables")
			verifyEnvironmentVariables := func(g Gomega) {
				// Get deployment env vars
				cmd := exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[*].name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("AGENT_NAME"), "Should contain template AGENT_NAME env var")
				g.Expect(output).To(ContainSubstring("PORT"), "Should contain user-defined PORT env var")
				g.Expect(output).To(ContainSubstring("LOG_LEVEL"), "Should contain user-defined LOG_LEVEL env var")
			}
			Eventually(verifyEnvironmentVariables).Should(Succeed())
		})

		It("should successfully deploy and manage the news agent (template)", func() {
			By("applying the news agent template sample")
			cmd := exec.Command("kubectl", "apply", "-f",
				"config/samples/runtime_v1alpha1_agent_template.yaml", "-n", testNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to apply news agent template sample")

			By("verifying the news agent resource is created")
			verifyAgentExists := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", newsAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyAgentExists).Should(Succeed())

			By("waiting for the news agent deployment to be created and ready")
			verifyDeploymentReady := func(g Gomega) {
				// Check deployment exists
				cmd := exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Check deployment is ready
				cmd = exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Deployment should have 1 ready replica")

				// Check deployment status
				cmd = exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Deployment should be available")
			}
			Eventually(verifyDeploymentReady, 3*time.Minute).Should(Succeed())

			By("verifying the news agent service is created")
			verifyServiceExists := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", newsAgentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Verify service has the expected port (google-adk default port 8000)
				cmd = exec.Command("kubectl", "get", "service", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.ports[0].port}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("8000"), "Service should expose port 8000")
			}
			Eventually(verifyServiceExists).Should(Succeed())

			By("verifying the news agent pod is healthy")
			verifyPodHealthy := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "-l", "app="+newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.items[0].status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "News agent pod should be running")

				// Check pod is ready
				cmd = exec.Command("kubectl", "get", "pods", "-l", "app="+newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.items[0].status.containerStatuses[0].ready}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "News agent pod should be ready")
			}
			Eventually(verifyPodHealthy, 2*time.Minute).Should(Succeed())

			By("verifying template image is set correctly")
			verifyTemplateImage := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].image}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(webhookv1alpha1.DefaultTemplateImageAdk), "Should use template image")
			}
			Eventually(verifyTemplateImage).Should(Succeed())

			By("verifying template environment variables are set correctly")
			verifyTemplateEnvironmentVariables := func(g Gomega) {
				// Get all environment variable names
				cmd := exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[*].name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Verify template-specific environment variables
				g.Expect(output).To(ContainSubstring("AGENT_NAME"), "Should contain AGENT_NAME")
				g.Expect(output).To(ContainSubstring("AGENT_DESCRIPTION"), "Should contain AGENT_DESCRIPTION")
				g.Expect(output).To(ContainSubstring("AGENT_INSTRUCTION"), "Should contain AGENT_INSTRUCTION")
				g.Expect(output).To(ContainSubstring("AGENT_MODEL"), "Should contain AGENT_MODEL")
				g.Expect(output).To(ContainSubstring("SUB_AGENTS"), "Should contain SUB_AGENTS")
				g.Expect(output).To(ContainSubstring("AGENT_TOOLS"), "Should contain AGENT_TOOLS")

				// Verify AGENT_MODEL value
				cmd = exec.Command("kubectl", "get", "deployment", newsAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='AGENT_MODEL')].value}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("gemini/gemini-2.5-flash"), "AGENT_MODEL should be set correctly")
			}
			Eventually(verifyTemplateEnvironmentVariables).Should(Succeed())
		})

		It("should verify successful reconciliation through metrics", func() {
			By("getting metrics to verify successful reconciliation")
			verifySuccessfulReconciliation := func(g Gomega) {
				metricsOutput := getMetricsOutput()
				g.Expect(metricsOutput).To(ContainSubstring(
					`controller_runtime_reconcile_total{controller="agent",result="success"}`,
				), "Should show successful agent reconciliation in metrics")
			}
			Eventually(verifySuccessfulReconciliation, 1*time.Minute).Should(Succeed())
		})

		It("should handle agent updates correctly", func() {
			By("updating the weather agent replica count")
			cmd := exec.Command("kubectl", "patch", "agent", weatherAgentName, "-n", testNamespace,
				"--type=merge", "-p", `{"spec":{"replicas":2}}`)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch weather agent")

			By("verifying deployment is updated with new replica count")
			verifyReplicaUpdate := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.replicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Deployment should be updated to 2 replicas")

				// Wait for replicas to be ready
				cmd = exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("2"), "Deployment should have 2 ready replicas")
			}
			Eventually(verifyReplicaUpdate, 2*time.Minute).Should(Succeed())
		})

		It("should resolve cluster-local subAgent references and populate SUB_AGENTS", func() {
			const subAgentName = "test-subagent"
			const parentAgentName = "test-parent-agent"

			By("creating a subAgent with A2A protocol")
			subAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: %s
  protocols:
    - type: A2A
      port: 8000
`, subAgentName, webhookv1alpha1.DefaultTemplateImageAdk)

			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(subAgentYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create subAgent")

			By("waiting for subAgent Status.Url to be populated")
			verifySubAgentStatus := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", subAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.url}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).NotTo(BeEmpty(), "SubAgent status URL should be set")
			}
			Eventually(verifySubAgentStatus).Should(Succeed())

			By("creating a parent agent that references the subAgent by name")
			parentAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: %s
  subAgents:
    - name: %s
      agentRef:
        name: %s
  protocols:
    - type: A2A
      port: 8000
`, parentAgentName, webhookv1alpha1.DefaultTemplateImageAdk, subAgentName, subAgentName)

			cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err = cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(parentAgentYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create parent agent")

			By("waiting for parent agent deployment to be ready")
			verifyParentDeploymentReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Parent deployment should have 1 ready replica")
			}
			Eventually(verifyParentDeploymentReady, 3*time.Minute).Should(Succeed())

			By("verifying SUB_AGENTS environment variable contains resolved URL")
			verifySUBAGENTSenv := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='SUB_AGENTS')].value}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(subAgentName), "SUB_AGENTS should contain subAgent name")
				g.Expect(output).To(ContainSubstring(fmt.Sprintf("%s.%s.svc.cluster.local", subAgentName, testNamespace)),
					"SUB_AGENTS should contain resolved cluster-local URL")
			}
			Eventually(verifySUBAGENTSenv).Should(Succeed())

			By("verifying parent agent status has Ready condition set to True")
			verifyStatusCondition := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Ready condition should be True")
			}
			Eventually(verifyStatusCondition).Should(Succeed())
		})

		It("should trigger reconciliation when subAgent status URL changes", func() {
			const subAgentName = "watch-subagent"
			const parentAgentName = "watch-parent"

			By("creating a subAgent with A2A protocol on port 8000")
			subAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: %s
  protocols:
    - type: A2A
      port: 8000
`, subAgentName, webhookv1alpha1.DefaultTemplateImageAdk)

			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(subAgentYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create subAgent")

			By("waiting for subAgent to be ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", subAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.url}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(":8000"), "SubAgent should have port 8000")
			}).Should(Succeed())

			By("creating parent agent referencing subAgent")
			parentAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: %s
  subAgents:
    - name: %s
      agentRef:
        name: %s
  protocols:
    - type: A2A
      port: 8000
`, parentAgentName, webhookv1alpha1.DefaultTemplateImageAdk, subAgentName, subAgentName)

			cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err = cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(parentAgentYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create parent agent")

			By("waiting for parent deployment to be ready and verify initial SUB_AGENTS")
			var initialSubAgentsValue string
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"))

				cmd = exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='SUB_AGENTS')].value}")
				subAgentsValue, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(subAgentsValue).To(ContainSubstring(":8000"))
				initialSubAgentsValue = subAgentsValue
			}, 3*time.Minute).Should(Succeed())

			By("updating subAgent to change port to 9000")
			cmd = exec.Command("kubectl", "patch", "agent", subAgentName, "-n", testNamespace,
				"--type=merge", "-p", `{"spec":{"protocols":[{"type":"A2A","port":9000}]}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch subAgent")

			By("waiting for subAgent status URL to update")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", subAgentName, "-n", testNamespace,
					"-o", "jsonpath={.status.url}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(":9000"), "SubAgent URL should now use port 9000")
			}).Should(Succeed())

			By("verifying parent deployment SUB_AGENTS was updated with new port")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='SUB_AGENTS')].value}")
				newSubAgentsValue, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(newSubAgentsValue).To(ContainSubstring(":9000"),
					"Parent SUB_AGENTS should be updated to port 9000")
				g.Expect(newSubAgentsValue).NotTo(Equal(initialSubAgentsValue),
					"SUB_AGENTS value should have changed")
			}, 2*time.Minute).Should(Succeed())
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
