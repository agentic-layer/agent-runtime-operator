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
})
