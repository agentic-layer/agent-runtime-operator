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
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

var _ = Describe("AgenticWorkforce", Ordered, func() {

	const (
		sampleFile       = "config/samples/runtime_v1alpha1_agenticworkforce.yaml"
		namespace1       = "test-wf-ns1"
		complexWorkforce = "complex-workforce"
		simpleWorkforce  = "simple-workforce"
	)

	BeforeAll(func() {
		By("applying the workforce sample")
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", sampleFile))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("cleaning up the workforce sample")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "-f", sampleFile, "--ignore-not-found=true"))
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			fetchControllerManagerPodLogs()
			fetchKubernetesEvents()
		}
	})

	It("should deploy and validate complex workforce with 3-level hierarchy", func() {
		By("verifying complex workforce is Ready")
		verifyWorkforceReady(complexWorkforce, namespace1)

		By("verifying complex workforce contains all transitive agents")
		verifyWorkforceContainsAgents(complexWorkforce, namespace1,
			"entry-agent", "mid-agent", "shared-agent")

		By("verifying complex workforce contains all transitive tools")
		verifyWorkforceContainsTools(complexWorkforce, namespace1,
			"toolserver-ns1", "toolserver-ns2")
	})

	It("should deploy and validate simple workforce with shared agent", func() {
		By("verifying simple workforce is Ready")
		verifyWorkforceReady(simpleWorkforce, namespace1)

		By("verifying simple workforce contains only the shared agent")
		verifyWorkforceContainsAgents(simpleWorkforce, namespace1, "shared-agent")

		By("verifying simple workforce contains only toolserver-ns1")
		verifyWorkforceContainsTools(simpleWorkforce, namespace1, "toolserver-ns1")
	})
})

// verifyWorkforceReady verifies that a workforce has the expected Ready status
func verifyWorkforceReady(name, namespace string) {
	timeout := 2 * time.Minute

	output, err := utils.Run(exec.Command("kubectl", "wait", "agenticworkforce", name, "-n", namespace,
		"--for=condition=Ready", "--timeout="+timeout.String()))
	Expect(err).NotTo(HaveOccurred(), func() string {
		return fmt.Sprintf("deployment is not ready (%s)", output)
	})
}

// verifyWorkforceContainsAgents verifies that a workforce's transitive agents contain the expected agents
func verifyWorkforceContainsAgents(name, namespace string, expectedAgents ...string) {
	output, err := utils.Run(exec.Command("kubectl", "get", "agenticworkforce", name, "-n", namespace,
		"-o", "jsonpath={.status.transitiveAgents[*].name}"))
	Expect(err).NotTo(HaveOccurred())
	for _, agent := range expectedAgents {
		Expect(output).To(ContainSubstring(agent), fmt.Sprintf("Should contain agent %s", agent))
	}
}

// verifyWorkforceContainsTools verifies that a workforce's transitive tools contain the expected tools
func verifyWorkforceContainsTools(name, namespace string, expectedTools ...string) {
	output, err := utils.Run(exec.Command("kubectl", "get", "agenticworkforce", name, "-n", namespace,
		"-o", "jsonpath={.status.transitiveTools}"))
	Expect(err).NotTo(HaveOccurred())
	for _, tool := range expectedTools {
		Expect(output).To(ContainSubstring(tool), fmt.Sprintf("Should contain tool %s", tool))
	}
}
