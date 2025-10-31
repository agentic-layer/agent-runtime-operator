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
	customFramework = "custom"
)

var _ = Describe("Agent Webhook", func() {
	var (
		obj       *runtimev1alpha1.Agent
		oldObj    *runtimev1alpha1.Agent
		validator AgentCustomValidator
		defaulter AgentCustomDefaulter
		ctx       context.Context
		recorder  *record.FakeRecorder
	)

	BeforeEach(func() {
		obj = &runtimev1alpha1.Agent{}
		oldObj = &runtimev1alpha1.Agent{}
		validator = AgentCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = AgentCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		ctx = context.Background()
		recorder = record.NewFakeRecorder(10)
		defaulter = AgentCustomDefaulter{
			DefaultReplicas:      1,
			DefaultPort:          8080,
			DefaultPortGoogleAdk: 8000,
			Recorder:             recorder,
		}
		validator = AgentCustomValidator{}
		obj = &runtimev1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
		}
	})

	Context("When creating Agent under Defaulting Webhook", func() {
		It("Should set default replicas when not specified", func() {
			By("having no replicas set initially")
			Expect(obj.Spec.Replicas).To(BeNil())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default replicas are set")
			Expect(obj.Spec.Replicas).NotTo(BeNil())
			Expect(*obj.Spec.Replicas).To(Equal(int32(1)))
		})

		It("Should not override existing replicas", func() {
			By("setting a custom replica count")
			replicas := int32(3)
			obj.Spec.Replicas = &replicas

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing replicas are preserved")
			Expect(*obj.Spec.Replicas).To(Equal(int32(3)))
		})

		It("Should set default port for google-adk framework", func() {
			By("setting up a google-adk agent with protocol but no port")
			obj.Spec.Framework = googleAdkFramework
			obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: runtimev1alpha1.A2AProtocol},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that google-adk default port is set")
			Expect(obj.Spec.Protocols).To(HaveLen(1))
			Expect(obj.Spec.Protocols[0].Port).To(Equal(int32(8000)))
		})

		It("Should set default port for other frameworks", func() {
			By("setting up an agent with unknown framework and protocol but no port")
			obj.Spec.Framework = customFramework
			obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: "OpenAI"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default port is set")
			Expect(obj.Spec.Protocols).To(HaveLen(1))
			Expect(obj.Spec.Protocols[0].Port).To(Equal(int32(8080)))
		})

		It("Should not override existing port", func() {
			By("setting up an agent with custom port")
			obj.Spec.Framework = googleAdkFramework
			obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: runtimev1alpha1.A2AProtocol, Port: 9000},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing port is preserved")
			Expect(obj.Spec.Protocols[0].Port).To(Equal(int32(9000)))
		})

		It("Should set default protocol name when not specified", func() {
			By("setting up a protocol without name")
			obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: runtimev1alpha1.A2AProtocol, Port: 8080},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that protocol name is generated with lowercase")
			Expect(obj.Spec.Protocols[0].Name).To(Equal("a2a-8080"))
		})

		It("Should not override existing protocol name", func() {
			By("setting up a protocol with custom name")
			obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
				{Type: runtimev1alpha1.A2AProtocol, Port: 8080, Name: "custom-name"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing name is preserved")
			Expect(obj.Spec.Protocols[0].Name).To(Equal("custom-name"))
		})

		It("Should allow user environment variables to override operator vars", func() {
			By("setting up environment variables including AGENT_NAME")
			obj.Spec.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
				{Name: "AGENT_NAME", Value: "user_defined_name"},
				{Name: "ANOTHER_VAR", Value: "another_value"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that all user environment variables are preserved")
			Expect(obj.Spec.Env).To(HaveLen(3))
			Expect(obj.Spec.Env[0].Name).To(Equal("CUSTOM_VAR"))
			Expect(obj.Spec.Env[1].Name).To(Equal("AGENT_NAME"))
			Expect(obj.Spec.Env[1].Value).To(Equal("user_defined_name"))
			Expect(obj.Spec.Env[2].Name).To(Equal("ANOTHER_VAR"))

			By("verifying that no filtering events were recorded")
			// Since we no longer filter environment variables, no events should be recorded
			Consistently(recorder.Events).ShouldNot(Receive())
		})

		It("Should not modify environment variables when no protected variables present", func() {
			By("setting up environment variables without protected ones")
			obj.Spec.Env = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
				{Name: "ANOTHER_VAR", Value: "another_value"},
			}

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that all environment variables are preserved")
			Expect(obj.Spec.Env).To(HaveLen(2))
			Expect(obj.Spec.Env[0].Name).To(Equal("CUSTOM_VAR"))
			Expect(obj.Spec.Env[1].Name).To(Equal("ANOTHER_VAR"))
		})
	})

	Context("When creating or updating Agent under Validating Webhook", func() {
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

				By("verifying that protocol list is populated with default protocol")
				Expect(emptyAgent.Spec.Protocols).To(HaveLen(1))
				Expect(emptyAgent.Spec.Protocols[0].Type).To(Equal(runtimev1alpha1.A2AProtocol))
				Expect(emptyAgent.Spec.Protocols[0].Name).To(Equal("a2a-8080"))
				Expect(emptyAgent.Spec.Protocols[0].Port).To(Equal(int32(8080)))
			})

			It("Should handle multiple protocols with mixed configurations", func() {
				By("setting up multiple protocols with different configurations")
				obj.Spec.Framework = googleAdkFramework
				obj.Spec.Protocols = []runtimev1alpha1.AgentProtocol{
					{Type: runtimev1alpha1.A2AProtocol},                             // No port, no name
					{Type: runtimev1alpha1.A2AProtocol, Port: 9000},                 // Port set, no name
					{Type: runtimev1alpha1.A2AProtocol, Name: "custom", Port: 8500}, // Both set
				}

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying defaults are applied correctly")
				Expect(obj.Spec.Protocols).To(HaveLen(3))

				Expect(obj.Spec.Protocols[0].Port).To(Equal(int32(8000))) // google-adk default
				Expect(obj.Spec.Protocols[0].Name).To(Equal("a2a-8000"))  // generated name with lowercase

				Expect(obj.Spec.Protocols[1].Port).To(Equal(int32(9000))) // preserved port
				Expect(obj.Spec.Protocols[1].Name).To(Equal("a2a-9000"))  // generated name with lowercase

				Expect(obj.Spec.Protocols[2].Port).To(Equal(int32(8500))) // preserved port
				Expect(obj.Spec.Protocols[2].Name).To(Equal("custom"))    // preserved name
			})
		})

		Context("When setting default images", func() {
			It("Should set template image for google-adk framework when no image specified", func() {
				By("setting up a google-adk agent without image")
				obj.Spec.Framework = googleAdkFramework

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying that template image is set")
				Expect(obj.Spec.Image).To(Equal(DefaultTemplateImageAdk))
			})

			It("Should set fallback image for unknown framework when no image specified", func() {
				By("setting up an unknown framework agent without image")
				obj.Spec.Framework = "unknown-framework"

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying that fallback image is set")
				Expect(obj.Spec.Image).To(Equal("invalid"))
			})

			It("Should not override existing image", func() {
				By("setting up an agent with custom image")
				obj.Spec.Framework = googleAdkFramework
				obj.Spec.Image = "ghcr.io/custom/my-agent:latest"

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying that custom image is preserved")
				Expect(obj.Spec.Image).To(Equal("ghcr.io/custom/my-agent:latest"))
			})
		})

		Context("When validating agents", func() {
			It("Should pass validation for google-adk framework without image (template agent)", func() {
				By("setting up a google-adk agent without image")
				obj.Spec.Framework = googleAdkFramework
				obj.Spec.Description = "Test template agent"
				obj.Spec.Instruction = "Test instruction"

				By("calling the ValidateCreate method")
				warnings, err := validator.ValidateCreate(ctx, obj)

				By("verifying that validation passes")
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should pass validation for any framework with custom image", func() {
				By("setting up a custom agent with custom image")
				obj.Spec.Framework = customFramework
				obj.Spec.Image = "ghcr.io/example/custom-agent:latest"

				By("calling the ValidateCreate method")
				warnings, err := validator.ValidateCreate(ctx, obj)

				By("verifying that validation passes")
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("Should fail validation for non-google-adk framework without image", func() {
				By("setting up a custom agent without image")
				obj.Spec.Framework = customFramework
				// No image specified

				By("calling the ValidateCreate method")
				warnings, err := validator.ValidateCreate(ctx, obj)

				By("verifying that validation fails")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("framework \"custom\" requires a custom image"))
				Expect(err.Error()).To(ContainSubstring("Template agents are only supported for \"google-adk\" framework"))
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
						Framework: "custom",
						// No image - should fail
					},
				}

				By("calling the ValidateUpdate method")
				warnings, err := validator.ValidateUpdate(ctx, oldAgent, newAgent)

				By("verifying that validation fails")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("framework \"custom\" requires a custom image"))
				Expect(warnings).To(BeEmpty())
			})

			It("Should not validate on delete", func() {
				By("calling the ValidateDelete method")
				warnings, err := validator.ValidateDelete(ctx, obj)

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
				obj.Spec.Framework = googleAdkFramework
				obj.Spec.Description = "A test news agent"
				obj.Spec.Instruction = "You are a news agent that summarizes articles"
				obj.Spec.Model = "gpt-4"
				obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
					{Name: "summarizer", Url: "https://example.com/summarizer.json"},
				}
				obj.Spec.Tools = []runtimev1alpha1.AgentTool{
					{Name: "news_fetcher", ToolServerRef: corev1.ObjectReference{Name: "news-fetcher-tool"}},
				}

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying that all template fields are preserved")
				Expect(obj.Spec.Description).To(Equal("A test news agent"))
				Expect(obj.Spec.Instruction).To(Equal("You are a news agent that summarizes articles"))
				Expect(obj.Spec.Model).To(Equal("gpt-4"))
				Expect(obj.Spec.SubAgents).To(HaveLen(1))
				Expect(obj.Spec.SubAgents[0].Name).To(Equal("summarizer"))
				Expect(obj.Spec.SubAgents[0].Url).To(Equal("https://example.com/summarizer.json"))
				Expect(obj.Spec.Tools).To(HaveLen(1))
				Expect(obj.Spec.Tools[0].Name).To(Equal("news_fetcher"))
				Expect(obj.Spec.Tools[0].ToolServerRef.Name).To(Equal("news-fetcher-tool"))

				By("verifying that template image is still set")
				Expect(obj.Spec.Image).To(Equal(DefaultTemplateImageAdk))
			})

			It("Should work with empty template fields", func() {
				By("setting up an agent without template fields")
				obj.Spec.Framework = googleAdkFramework
				// All template fields empty

				By("calling the Default method")
				err := defaulter.Default(ctx, obj)
				Expect(err).NotTo(HaveOccurred())

				By("verifying that empty fields remain empty")
				Expect(obj.Spec.Description).To(BeEmpty())
				Expect(obj.Spec.Instruction).To(BeEmpty())
				Expect(obj.Spec.Model).To(BeEmpty())
				Expect(obj.Spec.SubAgents).To(BeEmpty())
				Expect(obj.Spec.Tools).To(BeEmpty())

				By("verifying that template image is still set")
				Expect(obj.Spec.Image).To(Equal(DefaultTemplateImageAdk))
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

		Context("URL validation", func() {
			var validator *AgentCustomValidator

			BeforeEach(func() {
				validator = &AgentCustomValidator{}
				// Set valid framework and image to avoid framework validation errors
				obj.Spec.Framework = "custom"
				obj.Spec.Image = "ghcr.io/custom/agent:1.0.0"
			})

			Describe("SubAgent URL validation", func() {
				It("should accept valid HTTP and HTTPS URLs", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "external", Url: "https://example.com/agent.json"},
						{Name: "internal", Url: "http://agent-service:8080/config"},
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(BeEmpty())
				})

				It("should accept empty URLs", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "test",
							AgentRef: &corev1.ObjectReference{Name: "test"},
							Url:      ""}, // Empty URL should be allowed
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(BeEmpty())
				})

				It("should reject invalid URLs", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "test", Url: "not-a-valid-url"},
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("must use HTTP or HTTPS scheme"))
					Expect(warnings).To(BeEmpty())
				})

				It("should reject URLs without host", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "test", Url: "https://"},
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("must have a valid host"))
					Expect(warnings).To(BeEmpty())
				})

				It("should reject other schemes", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "test1", Url: "ftp://example.com/agent.json"},
						{Name: "test2", Url: "file:///path/to/agent.json"},
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("must use HTTP or HTTPS scheme"))
					Expect(warnings).To(BeEmpty())
				})

				It("should provide clear error messages with field paths", func() {
					obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
						{Name: "valid", Url: "https://example.com/valid"},
						{Name: "invalid", Url: "ftp://example.com/invalid"},
					}

					warnings, err := validator.validateAgent(obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.subAgents[1].url"))
					Expect(err.Error()).To(ContainSubstring("SubAgent[1].Url"))
					Expect(warnings).To(BeEmpty())
				})
			})
		})
	})

	Context("Volume validation", func() {
		BeforeEach(func() {
			validator = AgentCustomValidator{}
			obj.Spec.Framework = "custom"
			obj.Spec.Image = "ghcr.io/custom/agent:1.0.0"
		})

		Describe("Valid volume configurations", func() {
			It("should accept valid ConfigMap volume with volumeMount", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "config", MountPath: "/etc/config", ReadOnly: true},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept valid Secret volume with volumeMount", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "secrets",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "my-secret",
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "secrets", MountPath: "/etc/secrets", ReadOnly: true},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept EmptyDir volume", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "temp",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "temp", MountPath: "/tmp/data"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept PersistentVolumeClaim volume", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "my-pvc",
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "data", MountPath: "/data"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept multiple volumes and volumeMounts", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "config"},
							},
						},
					},
					{
						Name: "secrets",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{SecretName: "secrets"},
						},
					},
					{
						Name: "temp",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "config", MountPath: "/etc/config"},
					{Name: "secrets", MountPath: "/etc/secrets"},
					{Name: "temp", MountPath: "/tmp/cache"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept hostPath when explicitly allowed", func() {
				validator.AllowHostPath = true
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "host-data",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/data",
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "host-data", MountPath: "/host-data"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept agent with volumes but no volumeMounts", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "unused",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Describe("Invalid volumeMount references", func() {
			It("should reject volumeMount without corresponding volume", func() {
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "missing", MountPath: "/data"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("does not exist in spec.volumes"))
				Expect(err.Error()).To(ContainSubstring("missing"))
				Expect(warnings).To(BeEmpty())
			})

			It("should reject multiple volumeMounts with non-existent volumes", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "existing",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "existing", MountPath: "/existing"},
					{Name: "missing1", MountPath: "/missing1"},
					{Name: "missing2", MountPath: "/missing2"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing1"))
				Expect(err.Error()).To(ContainSubstring("missing2"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Describe("Mount path validation", func() {
			BeforeEach(func() {
				obj.Spec.Volumes = []corev1.Volume{
					{Name: "vol1", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					{Name: "vol2", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				}
			})

			It("should reject overlapping mount paths (nested paths)", func() {
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "vol1", MountPath: "/etc/config"},
					{Name: "vol2", MountPath: "/etc/config/subdir"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is nested under"))
				Expect(warnings).To(BeEmpty())
			})

			It("should reject overlapping mount paths (parent after child)", func() {
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "vol1", MountPath: "/etc/config/subdir"},
					{Name: "vol2", MountPath: "/etc/config"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("is nested under"))
				Expect(warnings).To(BeEmpty())
			})

			It("should accept similar but non-overlapping paths", func() {
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "vol1", MountPath: "/data"},
					{Name: "vol2", MountPath: "/data-backup"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept paths with trailing slashes correctly", func() {
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "vol1", MountPath: "/data/"},
					{Name: "vol2", MountPath: "/logs/"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Describe("Volume name validation", func() {
			It("should reject reserved volume name prefix", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "agent-operator-config",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "agent-operator-config", MountPath: "/config"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("reserved for operator use"))
				Expect(err.Error()).To(ContainSubstring("agent-operator-"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Describe("Volume source validation", func() {
			It("should reject hostPath by default", func() {
				validator.AllowHostPath = false
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "host", MountPath: "/host-root"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("hostPath volumes are not allowed"))
				Expect(err.Error()).To(ContainSubstring("security reasons"))
				Expect(warnings).To(BeEmpty())
			})
		})

		Describe("Complex validation scenarios", func() {
			It("should report multiple validation errors", func() {
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "agent-operator-reserved",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "missing-vol", MountPath: "/data1"},
					{Name: "agent-operator-reserved", MountPath: "/data2"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				// Should contain multiple error messages
				errMsg := err.Error()
				Expect(errMsg).To(ContainSubstring("reserved for operator use"))
				Expect(errMsg).To(ContainSubstring("does not exist in spec.volumes"))
				Expect(warnings).To(BeEmpty())
			})

			It("should validate volumes with valid framework and image", func() {
				obj.Spec.Framework = googleAdkFramework
				obj.Spec.Image = "" // Template agent
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "agent-config"},
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "config", MountPath: "/etc/config", ReadOnly: true},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should work with volumes alongside other agent features", func() {
				obj.Spec.SubAgents = []runtimev1alpha1.SubAgent{
					{Name: "helper", Url: "https://example.com/helper.json"},
				}
				obj.Spec.Tools = []runtimev1alpha1.AgentTool{
					{Name: "tool1", ToolServerRef: corev1.ObjectReference{Name: "tool1"}},
				}
				obj.Spec.Volumes = []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "data", MountPath: "/data"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})
	})
})
