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

var _ = Describe("ToolRoute Integration", Ordered, func() {
	const (
		sampleFile     = "config/samples/runtime_v1alpha1_toolroute.yaml"
		testNamespace  = "test-tool-servers"
		toolServerName = "example-http-toolserver"
		toolRouteName  = "example-toolroute"
		agentName      = "example-toolroute-agent"
		fakeGatewayURL = "http://fake-gateway.test-tool-servers.svc.cluster.local:8080/mcp"
	)

	BeforeAll(func() {
		By("applying the toolroute sample")
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", sampleFile))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply toolroute sample")

		By("patching ToolRoute status to simulate gateway assigning a URL")
		_, err = utils.Run(exec.Command("kubectl", "patch", "toolroute", toolRouteName,
			"-n", testNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"url":%q}}`, fakeGatewayURL),
		))
		Expect(err).NotTo(HaveOccurred(), "Failed to patch ToolRoute status with URL")
	})

	AfterAll(func() {
		By("cleaning up the toolroute sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", sampleFile, "--ignore-not-found=true"))
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()

			By("fetching events from the test namespace")
			eventsOutput, err := utils.Run(exec.Command("kubectl", "get", "events",
				"-n", testNamespace, "--sort-by=.lastTimestamp"))
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Events in %s:\n%s", testNamespace, eventsOutput)
			}
		}
	})

	It("should create an Agent Deployment whose AGENT_TOOLS env contains the ToolRoute status URL", func() {
		By("waiting for the Agent Deployment to exist")
		Eventually(func(g Gomega) {
			_, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", testNamespace))
			g.Expect(err).NotTo(HaveOccurred(), "Agent Deployment should exist")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("verifying AGENT_TOOLS env contains the ToolRoute URL")
		Eventually(func(g Gomega) {
			envOutput, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", testNamespace,
				"-o", `jsonpath={.spec.template.spec.containers[0].env}`))
			g.Expect(err).NotTo(HaveOccurred(), "Should be able to read Deployment env vars")
			g.Expect(envOutput).To(ContainSubstring(fakeGatewayURL),
				"AGENT_TOOLS env should contain the URL from ToolRoute.status.url")
			g.Expect(envOutput).To(ContainSubstring("example_tools"),
				"AGENT_TOOLS env should contain the tool name")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("should update the Agent Deployment when ToolRoute status URL changes", func() {
		const updatedURL = "http://updated-gateway.test-tool-servers.svc.cluster.local:8080/mcp/v2"

		By("patching ToolRoute status with a new URL")
		_, err := utils.Run(exec.Command("kubectl", "patch", "toolroute", toolRouteName,
			"-n", testNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"url":%q}}`, updatedURL),
		))
		Expect(err).NotTo(HaveOccurred(), "Failed to update ToolRoute status URL")

		By("verifying AGENT_TOOLS env is updated with the new URL")
		Eventually(func(g Gomega) {
			envOutput, err := utils.Run(exec.Command("kubectl", "get", "deployment", agentName,
				"-n", testNamespace,
				"-o", `jsonpath={.spec.template.spec.containers[0].env}`))
			g.Expect(err).NotTo(HaveOccurred(), "Should be able to read Deployment env vars")
			g.Expect(envOutput).To(ContainSubstring(updatedURL),
				"AGENT_TOOLS env should reflect the updated ToolRoute URL")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})
