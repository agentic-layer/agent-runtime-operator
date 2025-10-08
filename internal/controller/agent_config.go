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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// buildTemplateEnvironmentVars creates template environment variables from Agent spec fields.
// These template variables are always set regardless of whether using template or custom images,
// providing a consistent interface for agent configuration.
//
// Template variables created:
//   - AGENT_NAME: Always set to the agent's name
//   - AGENT_DESCRIPTION: Set to spec.Description (empty string if not specified)
//   - AGENT_INSTRUCTION: Set to spec.Instruction (empty string if not specified)
//   - AGENT_MODEL: Set to spec.Model (empty string if not specified)
//   - SUB_AGENTS: JSON-encoded map of sub-agent configurations (empty object if none)
//   - AGENT_TOOLS: JSON-encoded map of MCP tool configurations (empty object if none)
//
// JSON Structure:
//   - SubAgents: {"agentName": {"url": "https://..."}}
//   - Tools: {"toolName": {"url": "https://..."}}
//
// Parameters:
//   - agent: The Agent resource to generate template variables for
//   - resolvedSubAgents: Map of subAgent name to resolved URL (already validated)
//
// Returns:
//   - []corev1.EnvVar: Slice of environment variables for template configuration
//   - error: JSON marshaling error if SubAgents or Tools contain invalid data
func (r *AgentReconciler) buildTemplateEnvironmentVars(agent *runtimev1alpha1.Agent, resolvedSubAgents map[string]string) ([]corev1.EnvVar, error) {
	var templateEnvVars []corev1.EnvVar

	// Always set AGENT_NAME (sanitized to meet environment variable requirements)
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_NAME",
		Value: r.sanitizeAgentName(agent.Name),
	})

	// AGENT_DESCRIPTION - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_DESCRIPTION",
		Value: agent.Spec.Description,
	})

	// AGENT_INSTRUCTION - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_INSTRUCTION",
		Value: agent.Spec.Instruction,
	})

	// AGENT_MODEL - always set, even if empty
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_MODEL",
		Value: agent.Spec.Model,
	})

	// A2A_AGENT_CARD_URL - construct URL from A2A protocol if present
	a2aUrl := r.buildA2AAgentCardUrl(agent)
	if a2aUrl != "" {
		templateEnvVars = append(templateEnvVars, corev1.EnvVar{
			Name:  "A2A_AGENT_CARD_URL",
			Value: a2aUrl,
		})
	}

	// SUB_AGENTS - always set, with empty object if no subagents
	var subAgentsJSON []byte
	var err error
	if len(resolvedSubAgents) > 0 {
		subAgentsMap := make(map[string]map[string]string)
		for name, url := range resolvedSubAgents {
			subAgentsMap[r.sanitizeAgentName(name)] = map[string]string{
				"url": url,
			}
		}
		subAgentsJSON, err = json.Marshal(subAgentsMap)
	} else {
		subAgentsJSON, err = json.Marshal(map[string]interface{}{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subAgents: %w", err)
	}

	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "SUB_AGENTS",
		Value: string(subAgentsJSON),
	})

	// AGENT_TOOLS - always set, with empty object if no tools
	var toolsJSON []byte
	if len(agent.Spec.Tools) > 0 {
		toolsMap := make(map[string]map[string]string)
		for _, tool := range agent.Spec.Tools {
			toolsMap[tool.Name] = map[string]string{
				"url": tool.Url,
			}
		}
		toolsJSON, err = json.Marshal(toolsMap)
	} else {
		toolsJSON, err = json.Marshal(map[string]interface{}{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tools: %w", err)
	}
	templateEnvVars = append(templateEnvVars, corev1.EnvVar{
		Name:  "AGENT_TOOLS",
		Value: string(toolsJSON),
	})

	return templateEnvVars, nil
}

// mergeEnvironmentVariables merges template and user environment variables with proper precedence.
// User-defined environment variables override template variables with the same name, ensuring
// that users can customize agent behavior while maintaining template functionality.
//
// Merge Logic:
//  1. Start with all template variables
//  2. For each template variable, check if user has provided an override
//  3. If user override exists, use the user value instead of template value
//  4. Add any additional user variables that don't override template variables
//  5. Maintain original ordering where possible
//
// Example:
//
//	Template: [AGENT_NAME=test, AGENT_MODEL=default, TEMPLATE_VAR=value]
//	User:     [AGENT_MODEL=custom, USER_VAR=user]
//	Result:   [AGENT_NAME=test, AGENT_MODEL=custom, TEMPLATE_VAR=value, USER_VAR=user]
//
// Parameters:
//   - templateEnvVars: Environment variables generated from Agent template fields
//   - userEnvVars: Environment variables defined by user in Agent.Spec.Env
//
// Returns:
//   - []corev1.EnvVar: Merged environment variables with user precedence
func (r *AgentReconciler) mergeEnvironmentVariables(templateEnvVars, userEnvVars []corev1.EnvVar) []corev1.EnvVar {
	// Create a map for efficient lookups of user environment variables
	userEnvMap := make(map[string]corev1.EnvVar)
	for _, env := range userEnvVars {
		userEnvMap[env.Name] = env
	}

	// Pre-allocate result slice for efficiency
	result := make([]corev1.EnvVar, 0, len(templateEnvVars)+len(userEnvVars))

	// Add template environment variables, but skip if user has overridden them
	for _, templateEnv := range templateEnvVars {
		if userEnv, exists := userEnvMap[templateEnv.Name]; exists {
			// User has overridden this variable, use user's version
			result = append(result, userEnv)
			delete(userEnvMap, templateEnv.Name) // Remove so we don't add it again
		} else {
			// No user override, use template variable
			result = append(result, templateEnv)
		}
	}

	// Add any remaining user environment variables that weren't overrides
	for _, userEnv := range userEnvMap {
		result = append(result, userEnv)
	}

	return result
}

// sanitizeAgentName sanitizes the agent name to meet environment variable naming requirements.
// Environment variable names should start with a letter (a-z, A-Z) or underscore (_),
// and can only contain letters, digits (0-9), and underscores.
func (r *AgentReconciler) sanitizeAgentName(name string) string {
	var result strings.Builder

	// Process each character
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_':
			// Valid character, keep it
			result.WriteRune(r)
		case r == '-':
			// Convert hyphens to underscores
			result.WriteRune('_')
		default:
			// Replace any other character with underscore
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// Ensure it starts with a letter or underscore
	if len(sanitized) > 0 {
		firstChar := sanitized[0]
		if firstChar >= '0' && firstChar <= '9' {
			// Starts with digit, prepend underscore
			sanitized = "_" + sanitized
		}
	}

	// Ensure we have a valid result
	if sanitized == "" {
		sanitized = agentContainerName
	}

	return sanitized
}

// buildA2AAgentCardUrl constructs the fully qualified Kubernetes internal URL for the A2A agent card.
// The URL format is: http://{agent.Name}.{agent.Namespace}.svc.cluster.local:{port}/.well-known/agent-card.json
// Returns empty string if no A2A protocol is configured.
func (r *AgentReconciler) buildA2AAgentCardUrl(agent *runtimev1alpha1.Agent) string {
	// Find the A2A protocol
	for _, protocol := range agent.Spec.Protocols {
		if protocol.Type == "A2A" {
			return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s",
				agent.Name, agent.Namespace, protocol.Port, protocol.Path)
		}
	}
	return ""
}

// resolveSubAgentUrl resolves a SubAgent configuration to its actual URL.
// For remote agents (with URL): returns the URL directly
// For cluster agents (with agentRef): looks up the Agent resource and uses its status.url
// If namespace is not specified in agentRef, defaults to the parent agent's namespace
func (r *AgentReconciler) resolveSubAgentUrl(ctx context.Context, subAgent runtimev1alpha1.SubAgent, parentNamespace string) (string, error) {
	// If URL is provided, this is a remote agent - use URL directly
	if subAgent.Url != "" {
		return subAgent.Url, nil
	}

	// This is a cluster agent reference - resolve by looking up the Agent resource
	if subAgent.AgentRef == nil {
		return "", fmt.Errorf("subAgent has neither url nor agentRef specified")
	}

	// Use namespace from ObjectReference, or default to parent namespace
	namespace := subAgent.AgentRef.Namespace
	if namespace == "" {
		// Default to parent agent's namespace (following Kubernetes conventions)
		namespace = parentNamespace
	}

	var referencedAgent runtimev1alpha1.Agent
	err := r.Get(ctx, types.NamespacedName{
		Name:      subAgent.AgentRef.Name,
		Namespace: namespace,
	}, &referencedAgent)

	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster agent %s/%s: %w", namespace, subAgent.AgentRef.Name, err)
	}

	// Use the URL from the agent's status (populated by the controller)
	if referencedAgent.Status.Url == "" {
		return "", fmt.Errorf("cluster Agent %s/%s has no URL in its Status field (may not be ready or have A2A protocol)", namespace, subAgent.AgentRef.Name)
	}

	return referencedAgent.Status.Url, nil
}
