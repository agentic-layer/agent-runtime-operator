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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("ToolServer Deployment", Ordered, func() {
	const (
		sampleFile         = "config/samples/runtime_v1alpha1_toolserver.yaml"
		testNamespace      = "test-tool-servers"
		httpToolServerName = "example-http-toolserver"
	)

	BeforeAll(func() {
		By("applying the toolserver sample")
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", sampleFile, "-n", testNamespace))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply toolserver sample")
	})

	AfterAll(func() {
		By("cleaning up the toolserver sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", sampleFile, "-n", testNamespace, "--ignore-not-found=true"))
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()
		}
	})

	It("should successfully handle MCP requests", func() {
		By("sending an MCP tools/list request to the toolserver")
		Eventually(func(g Gomega) {
			tools, err := listMCPTools(httpToolServerName, testNamespace, 8080)
			g.Expect(err).NotTo(HaveOccurred(), "Should be able to list MCP tools")
			g.Expect(tools).NotTo(BeEmpty(), "Tool server should provide at least one tool")
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})

// listMCPTools sends an MCP tools/list request to the tool server and returns the list of tools
func listMCPTools(toolServerName, namespace string, port int) ([]interface{}, error) {
	// Construct MCP JSON-RPC payload for tools/list
	// Reference: https://spec.modelcontextprotocol.io/specification/server/tools/
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	// Some MCP servers require accepting both JSON and SSE even for HTTP transport
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}

	// Make the request with custom headers
	body, statusCode, err := utils.MakeServiceRequest(
		namespace, toolServerName, port,
		func(baseURL string) ([]byte, int, error) {
			return utils.PostRequest(baseURL+"/mcp", payload, headers)
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to send MCP request: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when sending MCP request: %s", statusCode, string(body))
	}

	// Handle SSE response format (event-stream)
	// SSE responses may start with "event: " or "data: " prefixes
	bodyStr := string(body)
	if len(bodyStr) > 0 && (bodyStr[0] == 'e' || bodyStr[0] == 'd') {
		// Parse SSE format: look for JSON data in the event stream
		// Format: "event: message\ndata: {...}\n\n"
		lines := string(body)
		dataPrefix := "data: "
		startIdx := 0
		for {
			dataIdx := strings.Index(lines[startIdx:], dataPrefix)
			if dataIdx == -1 {
				break
			}
			dataIdx += startIdx + len(dataPrefix)
			endIdx := strings.Index(lines[dataIdx:], "\n")
			if endIdx == -1 {
				endIdx = len(lines)
			} else {
				endIdx += dataIdx
			}
			jsonData := lines[dataIdx:endIdx]

			// Try to parse this data line as JSON
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &response); err == nil {
				// Successfully parsed JSON, check if it contains tools
				if result, ok := response["result"].(map[string]interface{}); ok {
					if tools, ok := result["tools"].([]interface{}); ok {
						return tools, nil
					}
				}
				// Check for error in this response
				if errObj, ok := response["error"]; ok {
					return nil, fmt.Errorf("MCP error response: %v", errObj)
				}
			}
			startIdx = endIdx + 1
			if startIdx >= len(lines) {
				break
			}
		}
		return nil, fmt.Errorf("no valid tool data found in SSE response")
	}

	// Handle standard JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MCP response: %w", err)
	}

	// Check for JSON-RPC error
	if errObj, ok := response["error"]; ok {
		return nil, fmt.Errorf("MCP error response: %v", errObj)
	}

	// Extract tools from result
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid MCP response format: missing result")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid MCP response format: missing tools array")
	}

	return tools, nil
}
