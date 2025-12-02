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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Config", func() {
	var reconciler *AgentReconciler

	BeforeEach(func() {
		reconciler = &AgentReconciler{}
	})

	Describe("buildTemplateEnvironmentVars", func() {
		It("should handle empty agent fields gracefully", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{},
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{},
				nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(envVars).To(HaveLen(6))

			// Verify all required template variables are present with correct values
			agentNameVar := findEnvVar(envVars, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_agent"))

			agentDescVar := findEnvVar(envVars, "AGENT_DESCRIPTION")
			Expect(agentDescVar).NotTo(BeNil())
			Expect(agentDescVar.Value).To(Equal(""))

			agentInstVar := findEnvVar(envVars, "AGENT_INSTRUCTION")
			Expect(agentInstVar).NotTo(BeNil())
			Expect(agentInstVar.Value).To(Equal(""))

			agentModelVar := findEnvVar(envVars, "AGENT_MODEL")
			Expect(agentModelVar).NotTo(BeNil())
			Expect(agentModelVar.Value).To(Equal(""))

			subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
			Expect(subAgentsVar).NotTo(BeNil())
			Expect(subAgentsVar.Value).To(Equal("{}"))

			toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
			Expect(toolsVar).NotTo(BeNil())
			Expect(toolsVar.Value).To(Equal("{}"))
		})

		It("should populate all template fields correctly", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "full-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Description: "Test agent description",
					Instruction: "You are a helpful assistant",
					Model:       "gpt-4",
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "sub1", Url: "https://example.com/sub1.json", InteractionType: "tool_call"},
						{Name: "sub2", Url: "https://example.com/sub2.json", InteractionType: "transfer"},
					},
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "tool1", ToolServerRef: corev1.ObjectReference{Name: "tool-server-1"}},
						{Name: "tool2", ToolServerRef: corev1.ObjectReference{Name: "tool-server-2"}},
					},
				},
			}

			// Simulate resolved subagents (URL-based subagents pass through directly)
			resolvedSubAgents := map[string]ResolvedSubAgent{
				"sub1": {Url: "https://example.com/sub1.json", InteractionType: "tool_call"},
				"sub2": {Url: "https://example.com/sub2.json", InteractionType: "transfer"},
			}

			// Simulate resolved tools (from ToolServer references)
			resolvedTools := map[string]string{
				"tool1": "http://tool-server-1.default.svc.cluster.local:8080/sse",
				"tool2": "http://tool-server-2.default.svc.cluster.local:8080/sse",
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, resolvedSubAgents, resolvedTools, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(envVars).To(HaveLen(6))

			agentDescVar := findEnvVar(envVars, "AGENT_DESCRIPTION")
			Expect(agentDescVar.Value).To(Equal("Test agent description"))

			agentInstVar := findEnvVar(envVars, "AGENT_INSTRUCTION")
			Expect(agentInstVar.Value).To(Equal("You are a helpful assistant"))

			agentModelVar := findEnvVar(envVars, "AGENT_MODEL")
			Expect(agentModelVar.Value).To(Equal("gpt-4"))

			subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
			Expect(subAgentsVar.Value).To(ContainSubstring("sub1"))
			Expect(subAgentsVar.Value).To(ContainSubstring("https://example.com/sub1.json"))
			Expect(subAgentsVar.Value).To(ContainSubstring("sub2"))
			Expect(subAgentsVar.Value).To(ContainSubstring("https://example.com/sub2.json"))

			toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
			Expect(toolsVar.Value).To(ContainSubstring("tool1"))
			Expect(toolsVar.Value).To(ContainSubstring("http://tool-server-1.default.svc.cluster.local:8080/sse"))
			Expect(toolsVar.Value).To(ContainSubstring("tool2"))
			Expect(toolsVar.Value).To(ContainSubstring("http://tool-server-2.default.svc.cluster.local:8080/sse"))
		})

		It("should handle JSON marshaling of complex structures", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "json-test-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					SubAgents: []runtimev1alpha1.SubAgent{
						{Name: "test-sub", Url: "https://example.com/sub.json", InteractionType: "tool_call"},
					},
				},
			}

			// Simulate resolved subagents (URL-based subagents pass through directly)
			resolvedSubAgents := map[string]ResolvedSubAgent{
				"test-sub": {Url: "https://example.com/sub.json", InteractionType: "tool_call"},
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, resolvedSubAgents, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify JSON structure is valid
			subAgentsVar := findEnvVar(envVars, "SUB_AGENTS")
			Expect(subAgentsVar.Value).To(MatchJSON(`{"test_sub":{"url":"https://example.com/sub.json","interaction_type":"tool_call"}}`))
		})

		It("should handle JSON marshaling of tools", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "json-test-agent-tools",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Tools: []runtimev1alpha1.AgentTool{
						{Name: "test-tool", ToolServerRef: corev1.ObjectReference{Name: "tool-server-1"}},
					},
				},
			}

			// Simulate resolved tools (from ToolServer references)
			resolvedTools := map[string]string{
				"test-tool": "http://tool-server-1.default.svc.cluster.local:8080/sse",
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, resolvedTools, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify JSON structure is valid
			toolsVar := findEnvVar(envVars, "AGENT_TOOLS")
			Expect(toolsVar.Value).To(MatchJSON(`{"test-tool":{"url":"http://tool-server-1.default.svc.cluster.local:8080/sse"}}`))
		})

		It("should generate AGENT_A2A_RPC_URL when A2A protocol is present", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a2a-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 8080,
							Path: "/api",
						},
					},
				},
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{},
				nil)
			Expect(err).NotTo(HaveOccurred())

			a2aUrlVar := findEnvVar(envVars, "AGENT_A2A_RPC_URL")
			Expect(a2aUrlVar).NotTo(BeNil())
			Expect(a2aUrlVar.Value).To(Equal("http://a2a-agent.test-ns.svc.cluster.local:8080/api"))
		})

		It("should not generate AGENT_A2A_RPC_URL when no A2A protocol is present", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-a2a-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{},
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{},
				nil)
			Expect(err).NotTo(HaveOccurred())

			a2aUrlVar := findEnvVar(envVars, "AGENT_A2A_RPC_URL")
			Expect(a2aUrlVar).To(BeNil())
		})
	})

	Context("when aiGatewayUrl is provided", func() {
		It("should set LiteLLM proxy environment variables", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{},
			}

			gatewayUrl := "http://ai-gateway.ai-gateway-ns.svc.cluster.local:4000"
			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{}, &gatewayUrl)
			Expect(err).NotTo(HaveOccurred())

			// Should have base variables (6) + LiteLLM variables (3) = 9 total
			Expect(envVars).To(HaveLen(9))

			// Verify LITELLM_PROXY_API_BASE is set
			proxyBaseVar := findEnvVar(envVars, "LITELLM_PROXY_API_BASE")
			Expect(proxyBaseVar).NotTo(BeNil())
			Expect(proxyBaseVar.Value).To(Equal("http://ai-gateway.ai-gateway-ns.svc.cluster.local:4000"))

			// Verify LITELLM_PROXY_API_KEY is set
			proxyKeyVar := findEnvVar(envVars, "LITELLM_PROXY_API_KEY")
			Expect(proxyKeyVar).NotTo(BeNil())
			Expect(proxyKeyVar.Value).To(Equal("NOT_USED_BY_GATEWAY"))

			// Verify USE_LITELLM_PROXY is set
			useLiteLLMVar := findEnvVar(envVars, "USE_LITELLM_PROXY")
			Expect(useLiteLLMVar).NotTo(BeNil())
			Expect(useLiteLLMVar.Value).To(Equal("True"))
		})

		It("should not set LiteLLM proxy variables when aiGatewayUrl is nil", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-gateway-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{},
			}

			envVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{}, nil)
			Expect(err).NotTo(HaveOccurred())

			// Should only have base variables (6), no LiteLLM variables
			Expect(envVars).To(HaveLen(6))

			// Verify LiteLLM variables are NOT present
			Expect(findEnvVar(envVars, "LITELLM_PROXY_API_BASE")).To(BeNil())
			Expect(findEnvVar(envVars, "LITELLM_PROXY_API_KEY")).To(BeNil())
			Expect(findEnvVar(envVars, "USE_LITELLM_PROXY")).To(BeNil())
		})

		It("should allow user to override LiteLLM proxy variables", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "override-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{},
			}

			gatewayUrl := "http://ai-gateway.default.svc.cluster.local:4000"
			templateVars, err := reconciler.buildTemplateEnvironmentVars(agent, map[string]ResolvedSubAgent{}, map[string]string{}, &gatewayUrl)
			Expect(err).NotTo(HaveOccurred())

			userVars := []corev1.EnvVar{
				{Name: "LITELLM_PROXY_API_KEY", Value: "custom-api-key"},
				{Name: "USE_LITELLM_PROXY", Value: "False"},
			}

			result := reconciler.mergeEnvironmentVariables(templateVars, userVars)

			// User overrides should take precedence
			proxyKeyVar := findEnvVar(result, "LITELLM_PROXY_API_KEY")
			Expect(proxyKeyVar).NotTo(BeNil())
			Expect(proxyKeyVar.Value).To(Equal("custom-api-key"))

			useLiteLLMVar := findEnvVar(result, "USE_LITELLM_PROXY")
			Expect(useLiteLLMVar).NotTo(BeNil())
			Expect(useLiteLLMVar.Value).To(Equal("False"))

			// Template variable that wasn't overridden should remain
			proxyBaseVar := findEnvVar(result, "LITELLM_PROXY_API_BASE")
			Expect(proxyBaseVar).NotTo(BeNil())
			Expect(proxyBaseVar.Value).To(Equal("http://ai-gateway.default.svc.cluster.local:4000"))
		})
	})

	Describe("mergeEnvironmentVariables", func() {
		It("should merge template and user variables with user precedence", func() {
			templateVars := []corev1.EnvVar{
				{Name: "AGENT_NAME", Value: "test_agent"},
				{Name: "AGENT_MODEL", Value: "default-model"},
				{Name: "TEMPLATE_ONLY", Value: "template-value"},
			}

			userVars := []corev1.EnvVar{
				{Name: "AGENT_MODEL", Value: "user-model"}, // Override
				{Name: "USER_ONLY", Value: "user-value"},   // New variable
			}

			result := reconciler.mergeEnvironmentVariables(templateVars, userVars)

			// Should have all unique variables
			Expect(result).To(HaveLen(4))

			// User variable should override template variable
			agentModelVar := findEnvVar(result, "AGENT_MODEL")
			Expect(agentModelVar).NotTo(BeNil())
			Expect(agentModelVar.Value).To(Equal("user-model"))

			// Template-only variable should remain
			templateOnlyVar := findEnvVar(result, "TEMPLATE_ONLY")
			Expect(templateOnlyVar).NotTo(BeNil())
			Expect(templateOnlyVar.Value).To(Equal("template-value"))

			// User-only variable should be present
			userOnlyVar := findEnvVar(result, "USER_ONLY")
			Expect(userOnlyVar).NotTo(BeNil())
			Expect(userOnlyVar.Value).To(Equal("user-value"))

			// Non-overridden template variable should remain
			agentNameVar := findEnvVar(result, "AGENT_NAME")
			Expect(agentNameVar).NotTo(BeNil())
			Expect(agentNameVar.Value).To(Equal("test_agent"))
		})

		It("should handle empty input slices", func() {
			// Empty template vars
			result := reconciler.mergeEnvironmentVariables([]corev1.EnvVar{}, []corev1.EnvVar{
				{Name: "USER_VAR", Value: "value"},
			})
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("USER_VAR"))

			// Empty user vars
			result = reconciler.mergeEnvironmentVariables([]corev1.EnvVar{
				{Name: "TEMPLATE_VAR", Value: "value"},
			}, []corev1.EnvVar{})
			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("TEMPLATE_VAR"))

			// Both empty
			result = reconciler.mergeEnvironmentVariables([]corev1.EnvVar{}, []corev1.EnvVar{})
			Expect(result).To(BeEmpty())
		})

		It("should sort environment variables alphabetically", func() {
			templateVars := []corev1.EnvVar{
				{Name: "VAR_C", Value: "c"},
				{Name: "VAR_A", Value: "a"},
				{Name: "VAR_B", Value: "b"},
			}

			userVars := []corev1.EnvVar{
				{Name: "VAR_B", Value: "b_override"},
				{Name: "VAR_D", Value: "d"},
			}

			result := reconciler.mergeEnvironmentVariables(templateVars, userVars)

			// Result should be sorted alphabetically by name
			Expect(result).To(HaveLen(4))
			Expect(result[0].Name).To(Equal("VAR_A"))
			Expect(result[0].Value).To(Equal("a"))
			Expect(result[1].Name).To(Equal("VAR_B"))
			Expect(result[1].Value).To(Equal("b_override")) // User override
			Expect(result[2].Name).To(Equal("VAR_C"))
			Expect(result[2].Value).To(Equal("c"))
			Expect(result[3].Name).To(Equal("VAR_D"))
			Expect(result[3].Value).To(Equal("d"))
		})
	})

	Describe("sanitizeAgentName", func() {
		DescribeTable("sanitize agent names",
			func(input, expected string) {
				result := reconciler.sanitizeAgentName(input)
				Expect(result).To(Equal(expected))
			},
			Entry("valid alphanumeric name", "test123", "test123"),
			Entry("name with hyphens", "test-agent-name", "test_agent_name"),
			Entry("name with underscores", "test_agent_name", "test_agent_name"),
			Entry("name starting with digit", "123test", "_123test"),
			Entry("name with special characters", "test@agent#name", "test_agent_name"),
			Entry("name with spaces", "test agent", "test_agent"),
			Entry("name with mixed case", "TestAgent", "TestAgent"),
			Entry("empty name", "", "agent"),
			Entry("only special characters", "@#$%", "____"),
			Entry("kubernetes-style name", "my-cool-agent-v2", "my_cool_agent_v2"),
		)
	})

	Describe("buildA2AAgentCardUrl", func() {
		It("should construct URL with default port and path", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 8000,
							Path: "",
						},
					},
				},
			}

			url := reconciler.buildA2AAgentCardUrl(agent)
			Expect(url).To(Equal("http://test-agent.test-ns.svc.cluster.local:8000/.well-known/agent-card.json"))
		})

		It("should construct URL with custom port", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 9090,
							Path: "",
						},
					},
				},
			}

			url := reconciler.buildA2AAgentCardUrl(agent)
			Expect(url).To(Equal("http://test-agent.test-ns.svc.cluster.local:9090/.well-known/agent-card.json"))
		})

		It("should construct URL with custom path", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 8000,
							Path: "/api/v1",
						},
					},
				},
			}

			url := reconciler.buildA2AAgentCardUrl(agent)
			Expect(url).To(Equal("http://test-agent.test-ns.svc.cluster.local:8000/api/v1/.well-known/agent-card.json"))
		})

		It("should return empty string when no A2A protocol is defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{},
				},
			}

			url := reconciler.buildA2AAgentCardUrl(agent)
			Expect(url).To(Equal(""))
		})

		It("should use first A2A protocol when multiple are defined", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "test-ns",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 8000,
							Path: "/first",
						},
						{
							Type: runtimev1alpha1.A2AProtocol,
							Port: 8001,
							Path: "/second",
						},
					},
				},
			}

			url := reconciler.buildA2AAgentCardUrl(agent)
			Expect(url).To(Equal("http://test-agent.test-ns.svc.cluster.local:8000/first/.well-known/agent-card.json"))
		})
	})
})
