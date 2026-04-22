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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("Agent with ToolRoute", Ordered, func() {
	const (
		toolRouteTestNamespace = "e2e-toolroute"
		toolServerName         = "ts-e2e"
		toolRouteName          = "tr-e2e"
		agentName              = "agent-e2e-toolroute"
		fakeGatewayURL         = "http://fake-gw/e2e/mcp"
	)

	BeforeAll(func() {
		By("creating the test namespace")
		// Use --dry-run=client + apply to make it idempotent
		_, err := utils.Run(exec.Command("kubectl", "create", "namespace", toolRouteTestNamespace, "--dry-run=client", "-o", "yaml"))
		Expect(err).NotTo(HaveOccurred())
		_, _ = utils.Run(exec.Command("kubectl", "create", "namespace", toolRouteTestNamespace))
		// Ignore error if namespace already exists

		By("applying the ToolServer resource")
		toolServerYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolServer
metadata:
  name: %s
  namespace: %s
spec:
  protocol: mcp
  transportType: http
  image: mcp/context7:latest
  port: 8080
  replicas: 1
`, toolServerName, toolRouteTestNamespace)
		_, err = utils.Run(kubectlApplyStdin(toolServerYAML))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply ToolServer")

		By("applying the ToolRoute resource")
		toolRouteYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolRoute
metadata:
  name: %s
  namespace: %s
spec:
  toolGatewayRef:
    name: placeholder-gateway
  upstream:
    toolServerRef:
      name: %s
`, toolRouteName, toolRouteTestNamespace, toolServerName)
		_, err = utils.Run(kubectlApplyStdin(toolRouteYAML))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply ToolRoute")

		By("patching ToolRoute status to simulate gateway assigning a URL")
		_, err = utils.Run(exec.Command("kubectl", "patch", "toolroute", toolRouteName,
			"-n", toolRouteTestNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"url":%q}}`, fakeGatewayURL),
		))
		Expect(err).NotTo(HaveOccurred(), "Failed to patch ToolRoute status with URL")

		By("applying the Agent resource referencing the ToolRoute")
		agentYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: %s
  namespace: %s
spec:
  framework: google-adk
  description: "E2E test agent with ToolRoute"
  instruction: "Test agent"
  model: "gemini/gemini-2.5-flash"
  tools:
    - name: e2e_tool
      toolRouteRef:
        name: %s
  protocols:
    - type: A2A
  replicas: 1
`, agentName, toolRouteTestNamespace, toolRouteName)
		_, err = utils.Run(kubectlApplyStdin(agentYAML))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Agent")
	})

	AfterAll(func() {
		By("cleaning up Agent resource")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "agent", agentName,
			"-n", toolRouteTestNamespace, "--ignore-not-found=true"))

		By("cleaning up ToolRoute resource")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "toolroute", toolRouteName,
			"-n", toolRouteTestNamespace, "--ignore-not-found=true"))

		By("cleaning up ToolServer resource")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "toolserver", toolServerName,
			"-n", toolRouteTestNamespace, "--ignore-not-found=true"))

		By("deleting the test namespace")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "namespace", toolRouteTestNamespace, "--ignore-not-found=true"))
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()

			By("fetching events from the test namespace")
			eventsOutput, err := utils.Run(exec.Command("kubectl", "get", "events",
				"-n", toolRouteTestNamespace, "--sort-by=.lastTimestamp"))
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Events in %s:\n%s", toolRouteTestNamespace, eventsOutput)
			}
		}
	})

	It("should create an Agent Deployment whose AGENT_TOOLS env contains the ToolRoute status URL", func() {
		By("waiting for the Agent Deployment to exist")
		Eventually(func(g Gomega) {
			_, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", toolRouteTestNamespace))
			g.Expect(err).NotTo(HaveOccurred(), "Agent Deployment should exist")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("verifying AGENT_TOOLS env contains the ToolRoute URL")
		Eventually(func(g Gomega) {
			envOutput, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", toolRouteTestNamespace,
				"-o", `jsonpath={.spec.template.spec.containers[0].env}`))
			g.Expect(err).NotTo(HaveOccurred(), "Should be able to read Deployment env vars")
			g.Expect(envOutput).To(ContainSubstring(fakeGatewayURL),
				"AGENT_TOOLS env should contain the URL from ToolRoute.status.url")
			g.Expect(envOutput).To(ContainSubstring("e2e_tool"),
				"AGENT_TOOLS env should contain the tool name")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should update the Agent Deployment when ToolRoute status URL changes", func() {
		const updatedURL = "http://fake-gw/e2e/mcp/v2"

		By("patching ToolRoute status with a new URL")
		_, err := utils.Run(exec.Command("kubectl", "patch", "toolroute", toolRouteName,
			"-n", toolRouteTestNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"url":%q}}`, updatedURL),
		))
		Expect(err).NotTo(HaveOccurred(), "Failed to update ToolRoute status URL")

		By("verifying AGENT_TOOLS env is updated with the new URL")
		Eventually(func(g Gomega) {
			envOutput, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", toolRouteTestNamespace,
				"-o", `jsonpath={.spec.template.spec.containers[0].env}`))
			g.Expect(err).NotTo(HaveOccurred(), "Should be able to read Deployment env vars")
			g.Expect(envOutput).To(ContainSubstring(updatedURL),
				"AGENT_TOOLS env should reflect the updated ToolRoute URL")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})

// kubectlApplyStdin returns a kubectl apply command that reads YAML from stdin.
// It writes the YAML to a temp file via the process substitution approach that
// works with exec.Command.
func kubectlApplyStdin(yamlContent string) *exec.Cmd {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yamlContent)
	return cmd
}
