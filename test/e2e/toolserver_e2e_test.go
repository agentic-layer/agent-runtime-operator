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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("ToolServer Deployment", Ordered, func() {
	const testNamespace = "default"
	const httpToolServerName = "example-http-toolserver"
	const stdioToolServerName = "example-stdio-toolserver"

	AfterAll(func() {
		By("cleaning up sample toolservers")
		cmd := exec.Command("kubectl", "delete", "-f", "config/samples/runtime_v1alpha1_toolserver_http.yaml",
			"-n", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		cmd = exec.Command("kubectl", "delete", "-f",
			"config/samples/runtime_v1alpha1_toolserver_stdio.yaml", "-n", testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()
		}
	})

	It("should successfully deploy and manage http transport toolserver", func() {
		By("applying the http toolserver sample")
		cmd := exec.Command("kubectl", "apply", "-f",
			"config/samples/runtime_v1alpha1_toolserver_http.yaml", "-n", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply http toolserver sample")

		By("verifying the http toolserver resource is created")
		verifyToolServerExists := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", httpToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}
		Eventually(verifyToolServerExists).Should(Succeed())

		By("verifying the http toolserver deployment is created")
		verifyDeploymentCreated := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", httpToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify deployment has correct replica count
			cmd = exec.Command("kubectl", "get", "deployment", httpToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.spec.replicas}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("2"), "Deployment should be configured with 2 replicas")
		}
		Eventually(verifyDeploymentCreated).Should(Succeed())

		By("verifying the http toolserver service is created")
		verifyServiceExists := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "service", httpToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "service", httpToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.spec.ports[0].port}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("8080"), "Service should expose port 8080")
		}
		Eventually(verifyServiceExists).Should(Succeed())

		By("verifying the http toolserver status URL is set")
		verifyStatusURL := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", httpToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.status.url}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			expectedURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080/mcp", httpToolServerName, testNamespace)
			g.Expect(output).To(Equal(expectedURL), "Status URL should be set correctly")
		}
		Eventually(verifyStatusURL, 1*time.Minute).Should(Succeed())

		By("verifying the http toolserver has Ready condition")
		verifyReadyCondition := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", httpToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "ToolServer should have Ready condition set to True")
		}
		Eventually(verifyReadyCondition, 1*time.Minute).Should(Succeed())

		By("verifying deployment has correct environment variables")
		verifyEnvironmentVariables := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", httpToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.spec.template.spec.containers[0].env[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("LOGLEVEL"), "Should contain LOGLEVEL env var")
		}
		Eventually(verifyEnvironmentVariables).Should(Succeed())
	})

	It("should successfully create stdio transport toolserver without deployment", func() {
		By("applying the stdio toolserver sample")
		cmd := exec.Command("kubectl", "apply", "-f",
			"config/samples/runtime_v1alpha1_toolserver_stdio.yaml", "-n", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply stdio toolserver sample")

		By("verifying the stdio toolserver resource is created")
		verifyToolServerExists := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", stdioToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}
		Eventually(verifyToolServerExists).Should(Succeed())

		By("verifying no deployment is created for stdio transport")
		verifyNoDeployment := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "deployment", stdioToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).To(HaveOccurred(), "Deployment should not exist for stdio transport")
		}
		Consistently(verifyNoDeployment, 30*time.Second, 5*time.Second).Should(Succeed())

		By("verifying no service is created for stdio transport")
		verifyNoService := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "service", stdioToolServerName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).To(HaveOccurred(), "Service should not exist for stdio transport")
		}
		Consistently(verifyNoService, 30*time.Second, 5*time.Second).Should(Succeed())

		By("verifying the stdio toolserver has Ready condition")
		verifyReadyCondition := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", stdioToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "ToolServer should have Ready condition set to True")
		}
		Eventually(verifyReadyCondition, 1*time.Minute).Should(Succeed())

		By("verifying the stdio toolserver status URL is empty")
		verifyStatusURL := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "toolserver", stdioToolServerName, "-n", testNamespace,
				"-o", "jsonpath={.status.url}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(BeEmpty(), "Status URL should be empty for stdio transport")
		}
		Eventually(verifyStatusURL, 1*time.Minute).Should(Succeed())
	})
})
