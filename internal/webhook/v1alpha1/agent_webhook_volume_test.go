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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("Agent Webhook - Volume Validation", func() {
	var (
		obj       *runtimev1alpha1.Agent
		validator AgentCustomValidator
	)

	BeforeEach(func() {
		validator = AgentCustomValidator{}
		obj = &runtimev1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
		}
		obj.Spec.Framework = "custom"
		obj.Spec.Image = "ghcr.io/custom/agent:1.0.0"
	})

	Context("Volume validation", func() {
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
						Name: "existing-vol",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
							},
						},
					},
				}
				obj.Spec.VolumeMounts = []corev1.VolumeMount{
					{Name: "missing-vol1", MountPath: "/data1"},
					{Name: "missing-vol2", MountPath: "/data2"},
				}

				warnings, err := validator.validateAgent(obj)
				Expect(err).To(HaveOccurred())
				// Should contain multiple error messages
				errMsg := err.Error()
				Expect(errMsg).To(ContainSubstring("does not exist in spec.volumes"))
				Expect(errMsg).To(ContainSubstring("missing-vol1"))
				Expect(errMsg).To(ContainSubstring("missing-vol2"))
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
					{Name: "tool1", ToolServerRef: &corev1.ObjectReference{Name: "tool1"}},
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
