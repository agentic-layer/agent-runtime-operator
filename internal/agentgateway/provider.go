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

package agentgateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	"github.com/agentic-layer/agent-runtime-operator/internal/agentgateway/krakend"
)

// AgentGatewayProvider defines the interface for gateway provider implementations
type AgentGatewayProvider interface {
	// CreateAgentGatewayResources creates all necessary resources for the agent gateway
	// including ConfigMaps, Deployments, Services, etc.
	CreateAgentGatewayResources(ctx context.Context, agentGateway *runtimev1alpha1.AgentGateway, exposedAgents []*runtimev1alpha1.Agent) error
}

// NewAgentGatewayProvider creates a new provider instance based on the provider type
func NewAgentGatewayProvider(providerType runtimev1alpha1.GatewayProvider, client client.Client, scheme *runtime.Scheme) (AgentGatewayProvider, error) {
	switch providerType {
	case runtimev1alpha1.KrakenDProvider:
		return krakend.NewProvider(client, scheme), nil
	case "":
		// Default to KrakenD if no provider specified
		return krakend.NewProvider(client, scheme), nil
	default:
		return nil, fmt.Errorf("unsupported gateway provider: %s", providerType)
	}
}
