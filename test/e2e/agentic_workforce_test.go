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

var _ = Describe("AgenticWorkforce Management", Ordered, func() {
	const testNamespace = "default"
	const testWorkforceName = "test-workforce"
	const testWorkforce2Name = "test-workforce-2"

	BeforeAll(func() {
		By("ensuring news-agent exists for workforce tests")
		newsAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: news-agent
spec:
  framework: google-adk
  description: "A news agent for testing"
  instruction: "You are a news agent"
  model: "gemini/gemini-2.5-flash"
  protocols:
    - type: A2A
      port: 8000
`
		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(newsAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create news-agent for workforce tests")

		By("waiting for news-agent to be ready")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", "news-agent", "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}, 1*time.Minute).Should(Succeed())

		By("ensuring weather-agent exists for workforce tests")
		weatherAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: weather-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/weather-agent:0.3.0
  protocols:
    - type: A2A
      port: 8000
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(weatherAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create weather-agent for workforce tests")

		By("waiting for weather-agent to be ready")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agent", "weather-agent", "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}, 1*time.Minute).Should(Succeed())
	})

	AfterAll(func() {
		By("cleaning up base agents used in workforce tests")
		cmd := exec.Command("kubectl", "delete", "agent", "news-agent", "weather-agent", "-n",
			testNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		By("cleaning up test workforces")
		testWorkforces := []string{testWorkforceName, testWorkforce2Name, "dynamic-workforce", "shared-deps-workforce"}
		for _, wfName := range testWorkforces {
			cmd := exec.Command("kubectl", "delete", "agenticworkforce", wfName, "-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		}

		By("cleaning up test agents created for workforce tests")
		testAgents := []string{"hierarchy-leaf", "hierarchy-mid", "hierarchy-entry", "missing-test-agent", "shared-sub"}
		for _, agentName := range testAgents {
			cmd := exec.Command("kubectl", "delete", "agent", agentName, "-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		}

		By("cleaning up test toolservers created for workforce tests")
		testToolServers := []string{"weather-tool", "news-tool"}
		for _, toolServerName := range testToolServers {
			cmd := exec.Command("kubectl", "delete", "toolserver", toolServerName,
				"-n", testNamespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		}
	})

	It("should create workforce with existing agents and show Ready status", func() {
		By("creating a workforce referencing news-agent")
		workforceYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgenticWorkforce
metadata:
  name: %s
spec:
  name: "Test Workforce"
  description: "E2E test workforce"
  owner: "test@example.com"
  entryPointAgents:
    - name: news-agent
`, testWorkforceName)

		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(workforceYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create workforce")

		By("verifying workforce resource is created")
		verifyWorkforceExists := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforceName, "-n", testNamespace)
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
		}
		Eventually(verifyWorkforceExists).Should(Succeed())

		By("verifying workforce status has Ready condition set to True")
		verifyReadyStatus := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforceName, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "Workforce should be Ready")
		}
		Eventually(verifyReadyStatus).Should(Succeed())

		By("verifying workforce status contains transitive agents")
		verifyTransitiveAgents := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforceName, "-n", testNamespace,
				"-o", "jsonpath={.status.transitiveAgents[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("news-agent"), "TransitiveAgents should contain news-agent")
		}
		Eventually(verifyTransitiveAgents).Should(Succeed())
	})

	It("should show Not Ready status when agents are deleted after workforce creation", func() {
		By("creating a temporary agent")
		tempAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: temp-agent-for-deletion
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/weather-agent:0.3.0
  protocols:
    - type: A2A
      port: 8000
`
		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(tempAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create temporary agent")

		By("creating a workforce referencing the temporary agent")
		workforceYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgenticWorkforce
metadata:
  name: %s
spec:
  name: "Test Workforce with Agent to be Deleted"
  description: "E2E test workforce for agent deletion scenario"
  owner: "test@example.com"
  entryPointAgents:
    - name: temp-agent-for-deletion
`, testWorkforce2Name)

		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(workforceYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create workforce")

		By("verifying workforce is initially Ready")
		verifyInitiallyReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforce2Name, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "Workforce should initially be Ready")
		}
		Eventually(verifyInitiallyReady, 30*time.Second).Should(Succeed())

		By("deleting the agent")
		cmd = exec.Command("kubectl", "delete", "agent", "temp-agent-for-deletion", "-n", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete agent")

		By("verifying workforce status has Ready condition set to False")
		verifyNotReadyStatus := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforce2Name, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("False"), "Workforce should not be Ready after agent deletion")
		}
		Eventually(verifyNotReadyStatus, 30*time.Second).Should(Succeed())

		By("verifying status message lists missing agents")
		verifyMissingAgentMessage := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforce2Name, "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].message}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("default/temp-agent-for-deletion"),
				"Message should list missing agent with namespace")
		}
		Eventually(verifyMissingAgentMessage, 30*time.Second).Should(Succeed())
	})

	It("should collect transitive agents and tools from hierarchy", func() {
		By("creating ToolServer resources for the leaf agent")
		weatherToolServerYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolServer
metadata:
  name: weather-tool
spec:
  protocol: mcp
  transportType: stdio
  image: ghcr.io/example/weather-tool:latest
`
		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(weatherToolServerYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create weather ToolServer")

		newsToolServerYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: ToolServer
metadata:
  name: news-tool
spec:
  protocol: mcp
  transportType: stdio
  image: ghcr.io/example/news-tool:latest
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(newsToolServerYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create news ToolServer")

		By("creating a leaf agent with tools")
		leafAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: hierarchy-leaf
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/template-agent-adk:0.3.0
  tools:
    - name: weather-tool
      toolServerRef:
        name: weather-tool
    - name: news-tool
      toolServerRef:
        name: news-tool
  protocols:
    - type: A2A
      port: 8000
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(leafAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create leaf agent")

		By("creating a mid-level agent that references leaf agent")
		midAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: hierarchy-mid
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/template-agent-adk:0.3.0
  subAgents:
    - name: leaf-agent
      agentRef:
        name: hierarchy-leaf
  protocols:
    - type: A2A
      port: 8000
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(midAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create mid-level agent")

		By("creating an entry agent that references mid-level agent")
		entryAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: hierarchy-entry
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/template-agent-adk:0.3.0
  subAgents:
    - name: mid-agent
      agentRef:
        name: hierarchy-mid
  protocols:
    - type: A2A
      port: 8000
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(entryAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create entry agent")

		By("creating a workforce referencing the entry agent")
		workforceYAML := fmt.Sprintf(`
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgenticWorkforce
metadata:
  name: %s
spec:
  name: "Hierarchical Workforce"
  description: "Test workforce with agent hierarchy"
  owner: "test@example.com"
  entryPointAgents:
    - name: hierarchy-entry
`, testWorkforceName)
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(workforceYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create workforce")

		By("verifying all three agents are in transitive agents")
		verifyAllAgents := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforceName, "-n", testNamespace,
				"-o", "jsonpath={.status.transitiveAgents[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("hierarchy-entry"), "Should contain entry agent")
			g.Expect(output).To(ContainSubstring("hierarchy-mid"), "Should contain mid-level agent")
			g.Expect(output).To(ContainSubstring("hierarchy-leaf"), "Should contain leaf agent")
		}
		Eventually(verifyAllAgents, 2*time.Minute).Should(Succeed())

		By("verifying tools from leaf agent are in transitive tools")
		verifyTransitiveTools := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", testWorkforceName, "-n", testNamespace,
				"-o", "jsonpath={.status.transitiveTools}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("weather-tool"), "Should contain weather tool")
			g.Expect(output).To(ContainSubstring("news-tool"), "Should contain news tool")
		}
		Eventually(verifyTransitiveTools, 2*time.Minute).Should(Succeed())
	})

	It("should update status when missing agent becomes available", func() {
		By("creating an agent initially")
		agentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: missing-test-agent
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/template-agent-adk:0.3.0
  protocols:
    - type: A2A
      port: 8000
`
		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(agentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create agent")

		By("creating a workforce referencing the agent")
		workforceYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgenticWorkforce
metadata:
  name: dynamic-workforce
spec:
  name: "Dynamic Workforce"
  description: "Test workforce for dynamic updates"
  owner: "test@example.com"
  entryPointAgents:
    - name: missing-test-agent
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(workforceYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create workforce")

		By("verifying workforce is Ready initially")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", "dynamic-workforce", "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"))
		}, 30*time.Second).Should(Succeed())

		By("deleting the agent to make workforce Not Ready")
		cmd = exec.Command("kubectl", "delete", "agent", "missing-test-agent", "-n", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete agent")

		By("verifying workforce becomes Not Ready")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", "dynamic-workforce", "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("False"))
		}, 30*time.Second).Should(Succeed())

		By("re-creating the agent")
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(agentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to re-create agent")

		By("verifying workforce status updates to Ready again")
		verifyBecameReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", "dynamic-workforce", "-n", testNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "Workforce should become Ready after agent is re-created")
		}
		Eventually(verifyBecameReady, 2*time.Minute).Should(Succeed())

		By("verifying transitive agents includes the agent")
		verifyTransitiveAgents := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", "dynamic-workforce", "-n", testNamespace,
				"-o", "jsonpath={.status.transitiveAgents[*].name}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(ContainSubstring("missing-test-agent"))
		}
		Eventually(verifyTransitiveAgents).Should(Succeed())
	})

	It("should deduplicate agents when multiple entry points share dependencies", func() {
		By("creating a shared sub-agent")
		sharedAgentYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: shared-sub
spec:
  framework: google-adk
  image: ghcr.io/agentic-layer/template-agent-adk:0.3.0
  protocols:
    - type: A2A
      port: 8000
`
		cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(sharedAgentYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create shared agent")

		By("updating weather-agent and news-agent to reference shared-sub")
		agentsToUpdate := []string{"weather-agent", "news-agent"}
		for _, agentName := range agentsToUpdate {
			cmd = exec.Command("kubectl", "patch", "agent", agentName, "-n", testNamespace,
				"--type=merge", "-p", `{"spec":{"subAgents":[{"name":"shared","agentRef":{"name":"shared-sub"}}]}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to patch %s", agentName))
		}

		By("creating a workforce with both entry points")
		workforceYAML := `
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: AgenticWorkforce
metadata:
  name: shared-deps-workforce
spec:
  name: "Shared Dependencies Workforce"
  description: "Test workforce with shared dependencies"
  owner: "test@example.com"
  entryPointAgents:
    - name: weather-agent
    - name: news-agent
`
		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		stdin, err = cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer func() { _ = stdin.Close() }()
			_, _ = stdin.Write([]byte(workforceYAML))
		}()
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create workforce")

		By("verifying all agents including shared one are in transitive agents")
		verifyDeduplication := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "agenticworkforce", "shared-deps-workforce", "-n", testNamespace,
				"-o", "json")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			// Parse JSON to count occurrences
			g.Expect(output).To(ContainSubstring("weather-agent"))
			g.Expect(output).To(ContainSubstring("news-agent"))
			g.Expect(output).To(ContainSubstring("shared-sub"))

			// Verify shared-sub appears exactly once in transitiveAgents array
			cmd = exec.Command("kubectl", "get", "agenticworkforce", "shared-deps-workforce", "-n", testNamespace,
				"-o", "jsonpath={.status.transitiveAgents[*].name}")
			transitiveAgents, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(transitiveAgents).To(ContainSubstring("shared-sub"), "Should contain shared agent")
		}
		Eventually(verifyDeduplication, 2*time.Minute).Should(Succeed())
	})
})
