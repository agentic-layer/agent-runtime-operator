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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	flokkFramework = "flokk"
)

var _ = Describe("Agent Webhook", func() {
	var (
		agent     *runtimev1alpha1.Agent
		defaulter *AgentCustomDefaulter
		validator *AgentCustomValidator
		ctx       context.Context
		recorder  *record.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		recorder = record.NewFakeRecorder(10)
		defaulter = &AgentCustomDefaulter{
			DefaultReplicas:      1,
			DefaultPort:          8080,
			DefaultPortGoogleAdk: 8000,
			Recorder:             recorder,
		}
		validator = &AgentCustomValidator{}
		agent = &runtimev1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
		}
	})

	Context("When applying defaults", func() {
		It("Should set default replicas when not specified", func() {
			By("having no replicas set initially")
			Expect(agent.Spec.Replicas).To(BeNil())

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default replicas are set")
			Expect(agent.Spec.Replicas).NotTo(BeNil())
			Expect(*agent.Spec.Replicas).To(Equal(int32(1)))
		})

		It("Should not override existing replicas", func() {
			By("setting a custom replica count")
			replicas := int32(3)
			agent.Spec.Replicas = &replicas

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing replicas are preserved")
			Expect(*agent.Spec.Replicas).To(Equal(int32(3)))
		})

		It("Should set default port for google-adk framework", func() {
			By("setting up a google-adk agent with protocol but no port")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "A2A"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that google-adk default port is set")
			Expect(agent.Spec.Protocols).To(HaveLen(1))
			Expect(agent.Spec.Protocols[0].Port).To(Equal(int32(8000)))
		})

		It("Should set default port for other frameworks", func() {
			By("setting up an agent with unknown framework and protocol but no port")
			agent.Spec.Framework = flokkFramework
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "OpenAI"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default port is set")
			Expect(agent.Spec.Protocols).To(HaveLen(1))
			Expect(agent.Spec.Protocols[0].Port).To(Equal(int32(8080)))
		})

		It("Should not override existing port", func() {
			By("setting up an agent with custom port")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "A2A", Port: 9000},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing port is preserved")
			Expect(agent.Spec.Protocols[0].Port).To(Equal(int32(9000)))
		})

		It("Should set default protocol name when not specified", func() {
			By("setting up a protocol without name")
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "A2A", Port: 8080},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that protocol name is generated with lowercase")
			Expect(agent.Spec.Protocols[0].Name).To(Equal("a2a-8080"))
		})

		It("Should not override existing protocol name", func() {
			By("setting up a protocol with custom name")
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "A2A", Port: 8080, Name: "custom-name"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing name is preserved")
			Expect(agent.Spec.Protocols[0].Name).To(Equal("custom-name"))
		})

		It("Should allow user environment variables to override operator vars", func() {
			By("setting up environment variables including AGENT_NAME")
			agent.Spec.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
				{Name: "AGENT_NAME", Value: "user_defined_name"},
				{Name: "ANOTHER_VAR", Value: "another_value"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that all user environment variables are preserved")
			Expect(agent.Spec.Env).To(HaveLen(3))
			Expect(agent.Spec.Env[0].Name).To(Equal("CUSTOM_VAR"))
			Expect(agent.Spec.Env[1].Name).To(Equal("AGENT_NAME"))
			Expect(agent.Spec.Env[1].Value).To(Equal("user_defined_name"))
			Expect(agent.Spec.Env[2].Name).To(Equal("ANOTHER_VAR"))

			By("verifying that no filtering events were recorded")
			// Since we no longer filter environment variables, no events should be recorded
			Consistently(recorder.Events).ShouldNot(Receive())
		})

		It("Should not modify environment variables when no protected variables present", func() {
			By("setting up environment variables without protected ones")
			agent.Spec.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
				{Name: "ANOTHER_VAR", Value: "another_value"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that all environment variables are preserved")
			Expect(agent.Spec.Env).To(HaveLen(2))
			Expect(agent.Spec.Env[0].Name).To(Equal("CUSTOM_VAR"))
			Expect(agent.Spec.Env[1].Name).To(Equal("ANOTHER_VAR"))
		})
	})

	Context("When handling invalid objects", func() {
		It("Should return error for non-Agent objects", func() {
			By("passing a non-Agent object")
			invalidObj := &corev1.Pod{}

			By("calling the Default method")
			err := defaulter.Default(ctx, invalidObj)

			By("verifying that an error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Agent object but got"))
		})

		It("Should handle nil runtime.Object gracefully", func() {
			By("passing a nil object")
			var nilObj runtime.Object

			By("calling the Default method")
			err := defaulter.Default(ctx, nilObj)

			By("verifying that an error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Agent object but got"))
		})
	})

	Context("When handling complex scenarios", func() {
		It("Should apply all defaults for completely empty agent", func() {
			By("having a completely empty agent spec")
			emptyAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-agent",
					Namespace: "default",
				},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, emptyAgent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default replicas are set")
			Expect(emptyAgent.Spec.Replicas).NotTo(BeNil())
			Expect(*emptyAgent.Spec.Replicas).To(Equal(int32(1)))

			By("verifying that protocols list is empty but handled gracefully")
			Expect(emptyAgent.Spec.Protocols).To(BeEmpty())
		})

		It("Should handle multiple protocols with mixed configurations", func() {
			By("setting up multiple protocols with different configurations")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "A2A"},                             // No port, no name
				{Type: "OpenAI", Port: 9000},              // Port set, no name
				{Type: "A2A", Name: "custom", Port: 8500}, // Both set
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying defaults are applied correctly")
			Expect(agent.Spec.Protocols).To(HaveLen(3))

			Expect(agent.Spec.Protocols[0].Port).To(Equal(int32(8000))) // google-adk default
			Expect(agent.Spec.Protocols[0].Name).To(Equal("a2a-8000"))  // generated name with lowercase

			Expect(agent.Spec.Protocols[1].Port).To(Equal(int32(9000)))   // preserved port
			Expect(agent.Spec.Protocols[1].Name).To(Equal("openai-9000")) // generated name with lowercase

			Expect(agent.Spec.Protocols[2].Port).To(Equal(int32(8500))) // preserved port
			Expect(agent.Spec.Protocols[2].Name).To(Equal("custom"))    // preserved name
		})
	})

	Context("When setting default images", func() {
		It("Should set template image for google-adk framework when no image specified", func() {
			By("setting up a google-adk agent without image")
			agent.Spec.Framework = googleAdkFramework

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that template image is set")
			Expect(agent.Spec.Image).To(Equal("ghcr.io/agentic-layer/agent-template-adk:0.1.0"))
		})

		It("Should set fallback image for unknown framework when no image specified", func() {
			By("setting up an unknown framework agent without image")
			agent.Spec.Framework = "unknown-framework"

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that fallback image is set")
			Expect(agent.Spec.Image).To(Equal("invalid"))
		})

		It("Should not override existing image", func() {
			By("setting up an agent with custom image")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Image = "ghcr.io/custom/my-agent:latest"

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that custom image is preserved")
			Expect(agent.Spec.Image).To(Equal("ghcr.io/custom/my-agent:latest"))
		})
	})

	Context("When validating agents", func() {
		It("Should pass validation for google-adk framework without image (template agent)", func() {
			By("setting up a google-adk agent without image")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Description = "Test template agent"
			agent.Spec.Instruction = "Test instruction"

			By("calling the ValidateCreate method")
			warnings, err := validator.ValidateCreate(ctx, agent)

			By("verifying that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should pass validation for any framework with custom image", func() {
			By("setting up a flokk agent with custom image")
			agent.Spec.Framework = flokkFramework
			agent.Spec.Image = "ghcr.io/example/flokk-agent:latest"

			By("calling the ValidateCreate method")
			warnings, err := validator.ValidateCreate(ctx, agent)

			By("verifying that validation passes")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should fail validation for non-google-adk framework without image", func() {
			By("setting up a flokk agent without image")
			agent.Spec.Framework = flokkFramework
			// No image specified

			By("calling the ValidateCreate method")
			warnings, err := validator.ValidateCreate(ctx, agent)

			By("verifying that validation fails")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("framework \"flokk\" requires a custom image"))
			Expect(err.Error()).To(ContainSubstring("Template agents are only supported for \"google-adk\" framework"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should fail validation for autogen framework without image", func() {
			By("setting up an autogen agent without image")
			agent.Spec.Framework = "autogen"
			// No image specified

			By("calling the ValidateCreate method")
			warnings, err := validator.ValidateCreate(ctx, agent)

			By("verifying that validation fails")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("framework \"autogen\" requires a custom image"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should handle ValidateUpdate correctly", func() {
			By("setting up old and new agent objects")
			oldAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: googleAdkFramework,
					Image:     "ghcr.io/old/image:v1",
				},
			}
			newAgent := &runtimev1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: runtimev1alpha1.AgentSpec{
					Framework: "flokk",
					// No image - should fail
				},
			}

			By("calling the ValidateUpdate method")
			warnings, err := validator.ValidateUpdate(ctx, oldAgent, newAgent)

			By("verifying that validation fails")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("framework \"flokk\" requires a custom image"))
			Expect(warnings).To(BeEmpty())
		})

		It("Should not validate on delete", func() {
			By("calling the ValidateDelete method")
			warnings, err := validator.ValidateDelete(ctx, agent)

			By("verifying that no validation is performed")
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("Should return error for non-Agent objects in validation", func() {
			By("passing a non-Agent object")
			invalidObj := &corev1.Pod{}

			By("calling the ValidateCreate method")
			warnings, err := validator.ValidateCreate(ctx, invalidObj)

			By("verifying that an error is returned")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an Agent object but got"))
			Expect(warnings).To(BeEmpty())
		})
	})

	Context("When handling template agent fields", func() {
		It("Should preserve all template fields", func() {
			By("setting up an agent with template fields")
			agent.Spec.Framework = googleAdkFramework
			agent.Spec.Description = "A test news agent"
			agent.Spec.Instruction = "You are a news agent that summarizes articles"
			agent.Spec.Model = "gpt-4"
			agent.Spec.SubAgents = []runtimev1alpha1.SubAgent{
				{Name: "summarizer", Url: "https://example.com/summarizer.json"},
			}
			agent.Spec.Tools = []runtimev1alpha1.AgentTool{
				{Name: "news_fetcher", Url: "https://news.mcpservers.org/mcp"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that all template fields are preserved")
			Expect(agent.Spec.Description).To(Equal("A test news agent"))
			Expect(agent.Spec.Instruction).To(Equal("You are a news agent that summarizes articles"))
			Expect(agent.Spec.Model).To(Equal("gpt-4"))
			Expect(agent.Spec.SubAgents).To(HaveLen(1))
			Expect(agent.Spec.SubAgents[0].Name).To(Equal("summarizer"))
			Expect(agent.Spec.SubAgents[0].Url).To(Equal("https://example.com/summarizer.json"))
			Expect(agent.Spec.Tools).To(HaveLen(1))
			Expect(agent.Spec.Tools[0].Name).To(Equal("news_fetcher"))
			Expect(agent.Spec.Tools[0].Url).To(Equal("https://news.mcpservers.org/mcp"))

			By("verifying that template image is still set")
			Expect(agent.Spec.Image).To(Equal("ghcr.io/agentic-layer/agent-template-adk:0.1.0"))
		})

		It("Should work with empty template fields", func() {
			By("setting up an agent without template fields")
			agent.Spec.Framework = googleAdkFramework
			// All template fields empty

			By("calling the Default method")
			err := defaulter.Default(ctx, agent)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that empty fields remain empty")
			Expect(agent.Spec.Description).To(BeEmpty())
			Expect(agent.Spec.Instruction).To(BeEmpty())
			Expect(agent.Spec.Model).To(BeEmpty())
			Expect(agent.Spec.SubAgents).To(BeEmpty())
			Expect(agent.Spec.Tools).To(BeEmpty())

			By("verifying that template image is still set")
			Expect(agent.Spec.Image).To(Equal("ghcr.io/agentic-layer/agent-template-adk:0.1.0"))
		})
	})

	Context("When sanitizing protocol names", func() {
		It("Should convert uppercase to lowercase", func() {
			result := sanitizeForPortName("OPENAI")
			Expect(result).To(Equal("openai"))
		})

		It("Should handle mixed case with special characters", func() {
			result := sanitizeForPortName("OpenAI_API")
			Expect(result).To(Equal("openai-api"))
		})

		It("Should remove leading and trailing hyphens", func() {
			result := sanitizeForPortName("_test_")
			Expect(result).To(Equal("test"))
		})

		It("Should handle empty string", func() {
			result := sanitizeForPortName("")
			Expect(result).To(Equal("port"))
		})

		It("Should handle string with only special characters", func() {
			result := sanitizeForPortName("!@#$%")
			Expect(result).To(Equal("port"))
		})

		It("Should preserve valid lowercase with hyphens", func() {
			result := sanitizeForPortName("a2a-protocol")
			Expect(result).To(Equal("a2a-protocol"))
		})

		It("Should handle numbers correctly", func() {
			result := sanitizeForPortName("HTTP2")
			Expect(result).To(Equal("http2"))
		})
	})
})
