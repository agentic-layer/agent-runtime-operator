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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Probe", func() {
	Describe("generateReadinessProbe", func() {
		It("should generate HTTP probe for A2A agents with default settings", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal("/" + agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(8000))
			Expect(probe.InitialDelaySeconds).To(Equal(int32(10)))
			Expect(probe.PeriodSeconds).To(Equal(int32(10)))
			Expect(probe.TimeoutSeconds).To(Equal(int32(3)))
			Expect(probe.SuccessThreshold).To(Equal(int32(1)))
			Expect(probe.FailureThreshold).To(Equal(int32(10)))
		})

		It("should return nil probe for agents with no protocols", func() {
			agent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-protocol-agent",
					Namespace: "default",
				},
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).To(BeNil())
		})

		It("should use custom port from protocol", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 3000, Path: "/a2a"},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(3000))
			Expect(probe.HTTPGet.Path).To(Equal("/a2a" + agentCardEndpoint))
		})

		It("should use custom path from A2A protocol", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Path: "/custom", Port: 8000},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal("/custom" + agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(8000))
		})

		It("should use both custom path and port from A2A protocol", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Path: "/agent-api", Port: 4000},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal("/agent-api" + agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(4000))
		})

		It("should handle empty path correctly", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: ""}, // Empty path
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal(agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(8000))
		})

		It("should handle custom port correctly", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 7000, Path: "/"},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(7000))
		})

		It("should use root path when path is set to '/'", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Path: "/", Port: 8000},
					},
				},
			}

			probe := generateReadinessProbe(agent)
			Expect(probe).NotTo(BeNil())
			Expect(probe.HTTPGet).NotTo(BeNil())
			Expect(probe.HTTPGet.Path).To(Equal("/" + agentCardEndpoint))
			Expect(probe.HTTPGet.Port.IntValue()).To(Equal(8000))
		})
	})

	Describe("findA2AProtocol", func() {
		It("should get A2A protocol correctly", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/"},
					},
				},
			}

			protocol := findA2AProtocol(agent)
			Expect(protocol).NotTo(BeNil())
			Expect(protocol.Type).To(Equal(runtimev1alpha1.A2AProtocol))
			Expect(protocol.Port).To(Equal(int32(8000)))
			Expect(protocol.Path).To(Equal("/"))
		})

		It("should return nil when A2A protocol not found", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{},
				},
			}

			protocol := findA2AProtocol(agent)
			Expect(protocol).To(BeNil())
		})

		It("should return first A2A protocol when multiple exist", func() {
			agent := &runtimev1alpha1.Agent{
				Spec: runtimev1alpha1.AgentSpec{
					Protocols: []runtimev1alpha1.AgentProtocol{
						{Type: runtimev1alpha1.A2AProtocol, Port: 8000, Path: "/first"},
						{Type: runtimev1alpha1.A2AProtocol, Port: 8001, Path: "/second"},
					},
				},
			}

			protocol := findA2AProtocol(agent)
			Expect(protocol).NotTo(BeNil())
			Expect(protocol.Port).To(Equal(int32(8000)))
			Expect(protocol.Path).To(Equal("/first"))
		})
	})
})
