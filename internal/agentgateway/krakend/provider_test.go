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

package krakend

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("KrakenD Provider", func() {
	var provider *Provider

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		provider = NewProvider(nil, scheme)
	})

	Describe("configMapNeedsUpdate", func() {
		It("should return false for identical ConfigMaps", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
				Data: map[string]string{
					"krakend.json": `{"version": 3, "port": 8080}`,
				},
			}

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
				Data: map[string]string{
					"krakend.json": `{"version": 3, "port": 8080}`,
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeFalse())
		})

		It("should return true when data differs", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3, "port": 8080}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3, "port": 9000}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when data key is missing", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
					"extra.json":   `{"extra": "data"}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when labels differ", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
						"version":  "v1.0.0",
					},
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when label value differs", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "envoy",
					},
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should handle empty labels correctly", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeFalse())
		})

		It("should handle nil labels correctly", func() {
			existing := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			}

			desired := &corev1.ConfigMap{
				Data: map[string]string{
					"krakend.json": `{"version": 3}`,
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			}

			result := provider.configMapNeedsUpdate(existing, desired)
			Expect(result).To(BeFalse())
		})
	})

	Describe("deploymentNeedsUpdate", func() {
		It("should return false for identical Deployments", func() {
			replicas := int32(2)
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":      "test",
								"provider": "krakend",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			desired := existing.DeepCopy()

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeFalse())
		})

		It("should return true when replica count differs", func() {
			replicas1 := int32(1)
			replicas2 := int32(3)

			existing := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas1,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			desired := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas2,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
			}

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when deployment labels differ", func() {
			replicas := int32(1)
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "krakend",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			desired := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "test",
						"provider": "envoy",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when template labels differ", func() {
			replicas := int32(1)
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":      "test",
								"provider": "krakend",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			desired := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":      "test",
								"provider": "envoy",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should return true when ConfigMap volume reference differs", func() {
			replicas := int32(1)
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "old-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			desired := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "new-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeTrue())
		})

		It("should handle nil replicas correctly", func() {
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: nil, // Should default to 1
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			desired := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: nil, // Should default to 1
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			result := provider.deploymentNeedsUpdate(existing, desired)
			Expect(result).To(BeFalse())
		})
	})

	Describe("getConfigMapNameFromVolumes", func() {
		It("should return ConfigMap name when found", func() {
			volumes := []corev1.Volume{
				{
					Name: "regular-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-config",
							},
						},
					},
				},
			}

			result := provider.getConfigMapNameFromVolumes(volumes)
			Expect(result).To(Equal("test-config"))
		})

		It("should return empty string when no ConfigMap volume found", func() {
			volumes := []corev1.Volume{
				{
					Name: "regular-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}

			result := provider.getConfigMapNameFromVolumes(volumes)
			Expect(result).To(Equal(""))
		})

		It("should return empty string for empty volumes", func() {
			volumes := []corev1.Volume{}

			result := provider.getConfigMapNameFromVolumes(volumes)
			Expect(result).To(Equal(""))
		})
	})
})
