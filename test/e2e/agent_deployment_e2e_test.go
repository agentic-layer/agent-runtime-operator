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
	"fmt"
	"os/exec"
	"time"

	"github.com/agentic-layer/agent-runtime-operator/internal/controller"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("Agent Deployment", Ordered, func() {
	const testNamespace = "default"
	const weatherAgentName = "weather-agent"
	const newsAgentName = "news-agent"
	const configMapName = "agent-config-map"

	BeforeAll(func() {
		By("applying sample configmap")
		cmd := exec.Command("kubectl", "apply", "-f", "config/samples/configmap.yaml",
			"-n", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply sample configmap")
	})

	AfterEach(func() {
		// After each test, check for failures and collect logs, events and pod descriptions for debugging.
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()
		}

		By("cleaning up test agents from individual tests")
		// Clean up test agents created in individual tests
		testAgents := []string{"test-subagent", "test-parent-agent", "watch-subagent", "watch-parent",
			"template-update-test-agent", "custom-image-test-agent",
			"interaction-type-subagent", "interaction-type-parent"}
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
			g.Expect(output).To(ContainSubstring("LOGLEVEL"), "Should contain user-defined LOGLEVEL env var")
		}
		Eventually(verifyEnvironmentVariables).Should(Succeed())

		By("verifying deployment has correct volume mounts")
		verifyVolumeMounts := func(g Gomega) {
			// Get volumeMounts from deployment
			cmd := exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("config-volume"), "Should contain config-volume volumeMount")

			// Verify volumeMount path
			cmd = exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[?(@.name=='config-volume')].mountPath}")
			output, err = utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("/etc/agent-config"), "Volume should be mounted at /etc/agent-config")
		}
		Eventually(verifyVolumeMounts).Should(Succeed())

		By("verifying deployment has correct volumes")
		verifyVolumes := func(g Gomega) {
			// Get volumes from deployment
			cmd := exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.volumes[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("config-volume"), "Should contain config-volume")

			// Verify volume references correct ConfigMap
			cmd = exec.Command("kubectl", "get", "deployment", weatherAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.volumes[?(@.name=='config-volume')].configMap.name}")
			output, err = utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal(configMapName), "Volume should reference correct ConfigMap")
		}
		Eventually(verifyVolumes).Should(Succeed())
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
			g.Expect(output).To(Equal(controller.DefaultTemplateImageAdk), "Should use template image")
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

	It("should NOT write default image to Agent CR spec for template agents", func() {
		const templateAgentName = "template-update-test-agent"

		By("creating a template agent without specifying an image")
		templateAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  description: "Test agent for validating template image updates"
  instruction: "You are a test agent"
  model: "gemini/gemini-2.5-flash"
  protocols:
    - type: A2A
      port: 8000
`, templateAgentName)

		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(templateAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create template agent")

		By("verifying the Agent CR does NOT have image field set (or is empty)")
		verifyImageNotSet := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(BeEmpty(),
				"Agent CR should not have image field set for template agents")
		}
		Eventually(verifyImageNotSet, 10*time.Second).Should(Succeed())
		Consistently(verifyImageNotSet, 5*time.Second).Should(Succeed())

		By("verifying the deployment DOES use a template image")
		verifyDeploymentImage := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).NotTo(BeEmpty(), "Deployment should have an image")
			g.Expect(output).To(ContainSubstring("agent-template-adk"),
				"Deployment should use the agent-template-adk image")
			g.Expect(output).To(ContainSubstring("ghcr.io/agentic-layer"),
				"Deployment should use an image from agentic-layer registry")
		}
		Eventually(verifyDeploymentImage, 30*time.Second).Should(Succeed())

		By("waiting for deployment to be ready")
		var originalImage string
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.status.readyReplicas}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("1"), "Deployment should have 1 ready replica")

			cmd = exec.Command("kubectl", "get", "deployment", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err = utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			originalImage = output
			g.Expect(originalImage).NotTo(BeEmpty())
		}, 3*time.Minute).Should(Succeed())

		By("updating the agent spec (changing description to trigger reconciliation)")
		cmd = exec.Command("kubectl", "patch", "agent", templateAgentName, "-n", testNamespace,
			"--type=merge", "-p", `{"spec":{"description":"Updated description to trigger reconciliation"}}`)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch agent")

		By("verifying the Agent CR still has no image field set after update")
		verifyImageStillNotSet := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(BeEmpty(),
				"Agent CR should still not have image field set after update")
		}
		Eventually(verifyImageStillNotSet, 10*time.Second).Should(Succeed())

		By("verifying the deployment continues using the same template image after update")
		verifyDeploymentImageUnchanged := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", templateAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal(originalImage),
				"Deployment should continue using the same template image")
		}
		Eventually(verifyDeploymentImageUnchanged, 30*time.Second).Should(Succeed())
	})

	It("should allow custom images to be set explicitly and preserve them", func() {
		const customAgentName = "custom-image-test-agent"

		By("creating an agent with an explicit custom image")
		customImageYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/weather-agent:0.3.0
  protocols:
    - type: A2A
`, customAgentName)

		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(customImageYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create custom image agent")

		By("verifying the Agent CR has the custom image set")
		verifyCustomImage := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", customAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("ghcr.io/agentic-layer/weather-agent:0.3.0"),
				"Agent CR should have the custom image set")
		}
		Eventually(verifyCustomImage).Should(Succeed())

		By("verifying the deployment uses the custom image")
		verifyDeploymentUsesCustomImage := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", customAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("ghcr.io/agentic-layer/weather-agent:0.3.0"),
				"Deployment should use the custom image")
		}
		Eventually(verifyDeploymentUsesCustomImage, 30*time.Second).Should(Succeed())

		By("updating the agent spec and verifying custom image is preserved")
		cmd = exec.Command("kubectl", "patch", "agent", customAgentName, "-n", testNamespace,
			"--type=merge", "-p", `{"spec":{"replicas":2}}`)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch custom agent")

		By("verifying the custom image is still set in the CR")
		Eventually(verifyCustomImage).Should(Succeed())

		By("verifying the deployment still uses the custom image")
		Eventually(verifyDeploymentUsesCustomImage, 30*time.Second).Should(Succeed())
	})

	It("should verify successful reconciliation", func() {
		By("verifying weather agent has Ready condition")
		verifyAgentReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", weatherAgentName, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "Weather agent should have Ready condition set to True")
		}
		Eventually(verifyAgentReady).Should(Succeed())

		By("verifying news agent has Ready condition")
		verifyNewsAgentReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", newsAgentName, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "News agent should have Ready condition set to True")
		}
		Eventually(verifyNewsAgentReady).Should(Succeed())
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
`, subAgentName, controller.DefaultTemplateImageAdk)

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
`, parentAgentName, controller.DefaultTemplateImageAdk, subAgentName, subAgentName)

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
`, subAgentName, controller.DefaultTemplateImageAdk)

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
`, parentAgentName, controller.DefaultTemplateImageAdk, subAgentName, subAgentName)

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

	It("should handle subAgent interactionType field (default and explicit)", func() {
		const subAgentName = "interaction-type-subagent"
		const parentAgentName = "interaction-type-parent"

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
`, subAgentName, controller.DefaultTemplateImageAdk)

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
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", subAgentName, "-n", testNamespace,
				"-o", "jsonpath={.status.url}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).NotTo(BeEmpty(), "SubAgent status URL should be set")
		}).Should(Succeed())

		By("creating parent agent WITHOUT specifying interactionType (should default to tool_call)")
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
`, parentAgentName, controller.DefaultTemplateImageAdk, subAgentName, subAgentName)

		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(parentAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create parent agent")

		By("waiting for parent deployment to be ready")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
				"-o", "jsonpath={.status.readyReplicas}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("1"), "Parent deployment should have 1 ready replica")
		}, 3*time.Minute).Should(Succeed())

		By("verifying SUB_AGENTS has default interaction_type 'tool_call'")
		var initialSubAgentsValue string
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='SUB_AGENTS')].value}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring(subAgentName), "SUB_AGENTS should contain subAgent name")
			g.Expect(output).To(ContainSubstring(`"interaction_type":"tool_call"`),
				"SUB_AGENTS should contain default interaction_type 'tool_call'")
			initialSubAgentsValue = output
		}).Should(Succeed())

		By("updating parent agent to set explicit interactionType: transfer")
		cmd = exec.Command("kubectl", "patch", "agent", parentAgentName, "-n", testNamespace,
			"--type=json", "-p",
			`[{"op":"replace","path":"/spec/subAgents/0/interactionType","value":"transfer"}]`)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch parent agent with interactionType")

		By("verifying deployment SUB_AGENTS was updated to interaction_type 'transfer'")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", parentAgentName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].env[?(@.name=='SUB_AGENTS')].value}")
			newSubAgentsValue, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(newSubAgentsValue).To(ContainSubstring(`"interaction_type":"transfer"`),
				"SUB_AGENTS should now contain interaction_type 'transfer'")
			g.Expect(newSubAgentsValue).NotTo(Equal(initialSubAgentsValue),
				"SUB_AGENTS value should have changed")
		}, 2*time.Minute).Should(Succeed())

		By("cleaning up test agents")
		cmd = exec.Command("kubectl", "delete", "agent", subAgentName, "-n", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "agent", parentAgentName, "-n", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	Context("Agent Volume Validation", func() {
		const testNamespace = "default"

		It("should reject agent with volumeMount referencing non-existent volume", func() {
			By("attempting to create agent with invalid volumeMount")
			invalidYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: invalid-mount-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  volumeMounts:
    - name: non-existent
      mountPath: /data
  protocols:
    - type: A2A
`
			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(invalidYAML))
			}()
			output, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred(), "Should reject volumeMount without volume")
			Expect(output).To(ContainSubstring("does not exist in spec.volumes"))
		})

		It("should reject agent with overlapping mount paths", func() {
			By("attempting to create agent with nested mount paths")
			overlappingYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: overlapping-mount-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  volumeMounts:
    - name: vol1
      mountPath: /etc/config
    - name: vol2
      mountPath: /etc/config/subdir
  volumes:
    - name: vol1
      emptyDir: {}
    - name: vol2
      emptyDir: {}
  protocols:
    - type: A2A
`
			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(overlappingYAML))
			}()
			output, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred(), "Should reject overlapping paths")
			Expect(output).To(ContainSubstring("is nested under"))
		})

		It("should reject agent with hostPath volume by default", func() {
			By("attempting to create agent with hostPath volume")
			hostPathYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: hostpath-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  volumeMounts:
    - name: host
      mountPath: /host-root
  volumes:
    - name: host
      hostPath:
        path: /
        type: Directory
  protocols:
    - type: A2A
`
			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(hostPathYAML))
			}()
			output, err := utils.Run(cmd)
			Expect(err).To(HaveOccurred(), "Should reject hostPath")
			Expect(output).To(ContainSubstring("hostPath volumes are not allowed"))
		})

		// Note: The webhook intentionally does NOT validate empty ConfigMap/Secret names
		// duplicate volume names, or duplicate mount paths, as these are already validated by Kubernetes when
		// creating the underlying Deployment. See PR #29 for discussion.

		It("should successfully update agent to remove volumes", func() {
			const agentName = "volume-update-agent"

			By("creating agent with volumes")
			agentWithVolumesYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  volumeMounts:
    - name: test-volume
      mountPath: /test-data
  volumes:
    - name: test-volume
      emptyDir: {}
  protocols:
    - type: A2A
`, agentName)

			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(agentWithVolumesYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should create agent with volumes")

			By("verifying deployment has the volume")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", agentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.volumes[?(@.name=='test-volume')].name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("test-volume"), "Deployment should have test-volume")
			}, 30*time.Second).Should(Succeed())

			By("updating agent to remove volumes")
			agentWithoutVolumesYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  protocols:
    - type: A2A
`, agentName)

			cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err = cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(agentWithoutVolumesYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should update agent to remove volumes")

			By("verifying deployment no longer has the volume")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", agentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.volumes[?(@.name=='test-volume')].name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty(), "Deployment should no longer have test-volume")
			}, 30*time.Second).Should(Succeed())

			By("verifying container no longer has the volume mount")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", agentName, "-n", testNamespace,
					"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[?(@.name=='test-volume')].name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty(), "Container should no longer have test-volume mount")
			}, 30*time.Second).Should(Succeed())

			By("cleaning up test agent")
			cmd = exec.Command("kubectl", "delete", "agent", agentName, "-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})

		It("should successfully deploy agent without volumes (backward compatibility)", func() {
			const agentName = "simple-backward-compat-agent"

			By("creating simple agent without any volume configuration")
			simpleAgentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/mock-agent:0.6
  protocols:
    - type: A2A
`, agentName)

			cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
			stdin, err := cmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
			go func() {
				defer func() { _ = stdin.Close() }()
				_, _ = stdin.Write([]byte(simpleAgentYAML))
			}()
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should create simple agent without volumes")

			By("verifying agent is created and ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "agent", agentName, "-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Agent should be ready")
			}, 2*time.Minute).Should(Succeed())

			By("verifying deployment is created and ready")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", agentName, "-n", testNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Deployment should be available")
			}, 2*time.Minute).Should(Succeed())

			By("verifying service is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", agentName, "-n", testNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, 30*time.Second).Should(Succeed())

			By("cleaning up test agent")
			cmd = exec.Command("kubectl", "delete", "agent", agentName, "-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})
	})
})
