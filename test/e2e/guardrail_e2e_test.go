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

package e2e

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("Guardrails", Ordered, func() {
	const (
		providerSampleFile = "config/samples/runtime_v1alpha1_guardrailprovider.yaml"
		guardSampleFile    = "config/samples/runtime_v1alpha1_guard.yaml"
		guardrailNamespace = "default"
	)

	BeforeAll(func() {
		By("applying the guardrailprovider sample")
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", providerSampleFile))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply guardrailprovider sample")

		By("applying the guard sample")
		_, err = utils.Run(exec.Command("kubectl", "apply", "-f", guardSampleFile))
		Expect(err).NotTo(HaveOccurred(), "Failed to apply guard sample")
	})

	AfterAll(func() {
		By("cleaning up the guard sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", guardSampleFile, "--ignore-not-found=true"))

		By("cleaning up the guardrailprovider sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", providerSampleFile, "--ignore-not-found=true"))
	})

	It("should successfully apply GuardrailProvider resources", func() {
		By("verifying openai-moderation provider exists")
		_, err := utils.Run(exec.Command("kubectl", "get", "guardrailprovider", "openai-moderation",
			"-n", guardrailNamespace))
		Expect(err).NotTo(HaveOccurred(), "openai-moderation GuardrailProvider should exist")

		By("verifying custom-openai-moderation provider exists")
		_, err = utils.Run(exec.Command("kubectl", "get", "guardrailprovider", "custom-openai-moderation",
			"-n", guardrailNamespace))
		Expect(err).NotTo(HaveOccurred(), "custom-openai-moderation GuardrailProvider should exist")

		By("verifying external-guardrail-provider exists")
		_, err = utils.Run(exec.Command("kubectl", "get", "guardrailprovider", "external-guardrail-provider",
			"-n", guardrailNamespace))
		Expect(err).NotTo(HaveOccurred(), "external-guardrail-provider GuardrailProvider should exist")
	})

	It("should successfully apply Guard resources", func() {
		By("verifying pii-guard exists")
		_, err := utils.Run(exec.Command("kubectl", "get", "guard", "pii-guard",
			"-n", guardrailNamespace))
		Expect(err).NotTo(HaveOccurred(), "pii-guard Guard should exist")

		By("verifying toxic-language-guard exists")
		_, err = utils.Run(exec.Command("kubectl", "get", "guard", "toxic-language-guard",
			"-n", guardrailNamespace))
		Expect(err).NotTo(HaveOccurred(), "toxic-language-guard Guard should exist")
	})
})
