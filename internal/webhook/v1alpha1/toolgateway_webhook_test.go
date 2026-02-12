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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolGateway Webhook", func() {
	var defaulter *ToolGatewayCustomDefaulter

	BeforeEach(func() {
		defaulter = &ToolGatewayCustomDefaulter{
			Recorder: &record.FakeRecorder{},
		}
	})

	Context("When applying defaults", func() {
		It("Should not modify ToolGatewayClassName when not specified", func() {
			gateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gateway",
				},
				Spec: runtimev1alpha1.ToolGatewaySpec{},
			}

			err := defaulter.Default(context.Background(), gateway)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway.Spec.ToolGatewayClassName).To(BeEmpty())
		})

		It("Should not override existing values", func() {
			gateway := &runtimev1alpha1.ToolGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-gateway",
				},
				Spec: runtimev1alpha1.ToolGatewaySpec{
					ToolGatewayClassName: "custom",
				},
			}

			err := defaulter.Default(context.Background(), gateway)
			Expect(err).NotTo(HaveOccurred())
			Expect(gateway.Spec.ToolGatewayClassName).To(Equal("custom"))
		})
	})
})
