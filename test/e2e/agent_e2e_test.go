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
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("Agents", Ordered, func() {

	const (
		sampleFile   = "config/samples/runtime_v1alpha1_agent.yaml"
		namespace    = "test-agents"
		simpleAgent  = "simple-agent"
		complexAgent = "complex-agent"
	)

	BeforeAll(func() {
		By("applying the agent sample")
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", sampleFile))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("cleaning up the agent sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", sampleFile, "--ignore-not-found=true"))
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()
		}
	})

	It("should deploy simple agent and verify basic functionality", func() {
		By("fetching simple-agent agent card")
		Eventually(func(g Gomega) {
			agentCard, err := fetchAgentCard(simpleAgent, namespace, 8000)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(agentCard).To(HaveKey("name"), "Agent card should contain name field")
			g.Expect(agentCard["name"]).To(Equal("mock_agent"), "Agent name should match")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("sending a message to simple-agent")
		response, err := sendAgentMessage(simpleAgent, namespace, 8000, "Hello, agent!")
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeEmpty(), "Agent should respond to messages")
	})

	It("should deploy complex agent with volume mounts and environment variables", func() {
		By("fetching complex-agent agent card on custom port 9000")
		var agentCard map[string]interface{}
		Eventually(func(g Gomega) {
			var err error
			agentCard, err = fetchAgentCard(complexAgent, namespace, 9000)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(agentCard).To(HaveKey("name"), "Agent card should contain name field")
			g.Expect(agentCard["name"]).To(Equal("mock_agent"), "Agent name should match")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		By("verifying agent card uses volume-mounted configuration")
		// The agent card is configured via the mounted ConfigMap
		Expect(agentCard).To(HaveKey("description"))
		Expect(agentCard["description"]).To(Equal("Mock Agent for the agent-runtime-operator E2E tests"))

		By("sending a message to complex-agent")
		response, err := sendAgentMessage(complexAgent, namespace, 9000, "Hello, complex agent!")
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeEmpty(), "Agent should respond to messages")
	})

	It("should handle agent configuration updates", func() {
		By("updating complex-agent port to 8080")
		_, err := utils.Run(exec.Command("kubectl", "patch", "agent", complexAgent, "-n", namespace,
			"--type=merge", "-p", `{"spec":{"protocols":[{"type":"A2A","port":8080}],
				"env":[{"name":"WIREMOCK_OPTIONS","value":"--port 8080 --permitted-system-keys AGENT_.*"}]}}`))
		Expect(err).NotTo(HaveOccurred())

		By("verifying agent is accessible on new port 8080")
		Eventually(func(g Gomega) {
			agentCard, err := fetchAgentCard(complexAgent, namespace, 8080)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(agentCard).To(HaveKey("name"))
		}, 1*time.Minute, 5*time.Second).Should(Succeed())

		By("verifying agent still responds to messages on new port")
		response, err := sendAgentMessage(complexAgent, namespace, 8080, "Hello after port change!")
		Expect(err).NotTo(HaveOccurred())
		Expect(response).NotTo(BeEmpty())
	})
})

// Helper functions for agent verification

// fetchAgentCard retrieves the agent card from the agent's well-known endpoint
func fetchAgentCard(agentName, namespace string, port int) (map[string]interface{}, error) {
	body, statusCode, err := utils.MakeServiceRequest(
		namespace, agentName, port,
		func(baseURL string) ([]byte, int, error) {
			return utils.GetRequest(baseURL + "/.well-known/agent-card.json")
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when fetching agent card: %s", statusCode, string(body))
	}

	var agentCard map[string]interface{}
	if err := json.Unmarshal(body, &agentCard); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent card: %w", err)
	}

	return agentCard, nil
}

// sendAgentMessage sends a message to the agent's RPC endpoint and returns the response
func sendAgentMessage(agentName, namespace string, port int, message string) (string, error) {
	// Construct A2A JSON-RPC payload
	// Reference: https://a2a-protocol.org/latest/specification/
	payload := map[string]interface{}{
		"id":      "test-123",
		"jsonrpc": "2.0",
		"method":  "message/send",
		"params": map[string]interface{}{
			"message": map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"text": message,
					},
				},
				"messageId": "e2e-test-message-id",
			},
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	body, statusCode, err := utils.MakeServiceRequest(
		namespace, agentName, port,
		func(baseURL string) ([]byte, int, error) {
			return utils.PostRequest(baseURL, payload, headers)
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to send message to agent: %w", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d when sending message: %s", statusCode, string(body))
	}

	return string(body), nil
}
