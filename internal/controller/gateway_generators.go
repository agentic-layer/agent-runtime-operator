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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

// GatewayConfigGenerator defines the interface for generating gateway configurations
type GatewayConfigGenerator interface {
	// Generate creates the configuration for the gateway provider
	Generate(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, agents []*runtimev1alpha1.Agent) (configData string, configHash string, err error)

	// Validate checks if the gateway configuration is valid for this provider
	Validate(gateway *runtimev1alpha1.AgentGateway) error

	// GetDefaultConfig returns the default configuration template
	GetDefaultConfig() map[string]interface{}
}

// NewGatewayGenerator creates a KrakenD gateway generator
// Currently only KrakenD is supported
func NewGatewayGenerator(provider runtimev1alpha1.GatewayProvider) (GatewayConfigGenerator, error) {
	switch provider {
	case runtimev1alpha1.KrakenDProvider, "": // Default to KrakenD
		return &KrakenDGenerator{}, nil
	default:
		return nil, fmt.Errorf("unsupported gateway provider '%s': only 'krakend' is currently supported", provider)
	}
}

// KrakenDGenerator implements KrakenD configuration generation
type KrakenDGenerator struct{}

// GetDefaultConfig returns KrakenD default configuration template
func (g *KrakenDGenerator) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"version":                       3,
		"port":                          8080,
		"timeout":                       "5s",
		"cache_ttl":                     "300s",
		"idle_timeout":                  "25s",
		"read_timeout":                  "0s",
		"write_timeout":                 "0s",
		"read_header_timeout":           "0s",
		"idle_connection_timeout":       "0s",
		"response_header_timeout":       "2s",
		"expect_continue_timeout":       "1s",
		"dialer_timeout":                "5s",
		"dialer_fallback_delay":         "300ms",
		"dialer_keep_alive":             "30s",
		"disable_keep_alives":           false,
		"disable_compression":           false,
		"max_idle_connections":          100,
		"max_idle_connections_per_host": 25,
		"endpoints":                     []map[string]interface{}{},
	}
}

// Validate checks KrakenD-specific configuration
func (g *KrakenDGenerator) Validate(gateway *runtimev1alpha1.AgentGateway) error {
	if gateway.Spec.Domain == "" {
		return fmt.Errorf("domain is required for KrakenD gateway")
	}

	if len(gateway.Spec.Agents) == 0 {
		return fmt.Errorf("at least one agent must be specified")
	}

	// Validate custom config values
	if gateway.Spec.Gateway.Config != nil {
		allowedKeys := map[string]bool{
			"timeout": true, "cache_ttl": true, "idle_timeout": true,
			"read_timeout": true, "write_timeout": true, "read_header_timeout": true,
			"max_idle_connections": true, "max_idle_connections_per_host": true,
		}

		for key := range gateway.Spec.Gateway.Config {
			if !allowedKeys[key] {
				return fmt.Errorf("unsupported KrakenD config key: %s", key)
			}
		}
	}

	return nil
}

// Generate creates KrakenD JSON configuration
func (g *KrakenDGenerator) Generate(ctx context.Context, gateway *runtimev1alpha1.AgentGateway, agents []*runtimev1alpha1.Agent) (string, string, error) {
	log := logf.FromContext(ctx)

	// Start with default configuration
	config := g.GetDefaultConfig()

	// Set gateway-specific values
	config["name"] = fmt.Sprintf("agent-gateway-%s", gateway.Name)
	config["host"] = []string{"https://" + gateway.Spec.Domain}

	// Merge custom configuration values
	if gateway.Spec.Gateway.Config != nil {
		for key, value := range gateway.Spec.Gateway.Config {
			config[key] = value
		}
	}

	// Generate endpoints for each enabled agent
	endpoints := []map[string]interface{}{}

	for i, agent := range agents {
		if i >= len(gateway.Spec.Agents) {
			continue
		}

		agentRef := gateway.Spec.Agents[i]
		if !agentRef.Enabled {
			continue
		}

		// Get target protocol with improved port handling
		protocol := getTargetProtocolForAgent(agent, agentRef.TargetProtocol)
		if protocol == nil {
			log.Info("No valid protocol found for agent", "agent", agent.Name)
			continue
		}

		port := protocol.Port
		if port == 0 {
			// Use Agent's default port if available, otherwise fallback to protocol defaults
			port = getDefaultPortForProtocol(protocol.Type)
			log.Info("Using default port for agent", "agent", agent.Name, "protocol", protocol.Type, "port", port)
		}

		// Determine namespace
		namespace := agentRef.Namespace
		if namespace == "" {
			namespace = gateway.Namespace
		}

		// Build upstream URL
		serviceName := fmt.Sprintf("%s.%s.svc.cluster.local", agent.Name, namespace)
		upstreamURL := fmt.Sprintf("http://%s:%d", serviceName, port)
		if protocol.Path != "" {
			upstreamURL += protocol.Path
		}

		// Determine endpoint path based on routing strategy
		endpointPath := g.getEndpointPath(gateway, agentRef)

		// Determine HTTP methods - use configured methods or protocol defaults
		methods := g.getHTTPMethods(agentRef, protocol)

		// Create endpoints for each HTTP method
		for _, method := range methods {
			endpoint := map[string]interface{}{
				"endpoint": endpointPath,
				"method":   method,
				"backend": []map[string]interface{}{
					{
						"url_pattern": "/",
						"host":        []string{upstreamURL},
						"method":      method,
					},
				},
			}

			// Add endpoint-specific configurations
			if gateway.Spec.Gateway.Config != nil {
				if timeout, ok := gateway.Spec.Gateway.Config["endpoint_timeout"]; ok {
					endpoint["timeout"] = timeout
				}
				if cacheTTL, ok := gateway.Spec.Gateway.Config["endpoint_cache_ttl"]; ok {
					endpoint["cache_ttl"] = cacheTTL
				}
			}

			endpoints = append(endpoints, endpoint)
		}
	}

	config["endpoints"] = endpoints

	// Generate JSON configuration
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal KrakenD config: %w", err)
	}

	// Generate configuration hash
	hash := sha256.Sum256(configJSON)
	configHash := fmt.Sprintf("%x", hash)[:16]

	return string(configJSON), configHash, nil
}

// getEndpointPath determines the endpoint path based on routing strategy
func (g *KrakenDGenerator) getEndpointPath(gateway *runtimev1alpha1.AgentGateway, agentRef runtimev1alpha1.AgentReference) string {
	switch gateway.Spec.RoutingStrategy {
	case runtimev1alpha1.PathBasedRouting:
		if !strings.HasPrefix(agentRef.RoutePrefix, "/") {
			return "/" + agentRef.RoutePrefix + "/*"
		}
		return agentRef.RoutePrefix + "/*"
	case runtimev1alpha1.SubdomainBasedRouting:
		// For subdomain routing, KrakenD handles all paths under each subdomain
		return "/*"
	default:
		return agentRef.RoutePrefix + "/*"
	}
}

// getHTTPMethods determines allowed HTTP methods for an agent
func (g *KrakenDGenerator) getHTTPMethods(agentRef runtimev1alpha1.AgentReference, protocol *runtimev1alpha1.AgentProtocol) []string {
	// Use configured methods if available
	if len(agentRef.AllowedMethods) > 0 {
		return agentRef.AllowedMethods
	}

	// Fallback to protocol-based defaults
	switch protocol.Type {
	case "OpenAI":
		return []string{"POST", "GET"} // OpenAI APIs typically use POST for completions, GET for models
	case "A2A":
		return []string{"GET", "POST"} // Agent-to-Agent might need both
	default:
		return []string{"GET"} // Conservative default
	}
}

// getTargetProtocolForAgent returns the target protocol for an agent
func getTargetProtocolForAgent(agent *runtimev1alpha1.Agent, targetProtocol string) *runtimev1alpha1.AgentProtocol {
	if targetProtocol != "" {
		for _, protocol := range agent.Spec.Protocols {
			if protocol.Type == targetProtocol {
				return &protocol
			}
		}
		return nil
	}

	// Return first protocol if no specific target
	if len(agent.Spec.Protocols) > 0 {
		return &agent.Spec.Protocols[0]
	}

	return nil
}

// getDefaultPortForProtocol returns sensible default ports for protocols
func getDefaultPortForProtocol(protocolType string) int32 {
	defaults := map[string]int32{
		"A2A":    8080,
		"OpenAI": 8000,
		"HTTP":   8080,
		"HTTPS":  8443,
		"gRPC":   9090,
	}

	if port, exists := defaults[protocolType]; exists {
		return port
	}

	return 8080 // Ultimate fallback
}
