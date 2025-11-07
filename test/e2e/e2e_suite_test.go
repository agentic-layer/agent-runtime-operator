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
	"os"
	"os/exec"
	"testing"
	"time"

	webhookv1alpha1 "github.com/agentic-layer/agent-runtime-operator/internal/webhook/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agentic-layer/agent-runtime-operator/test/utils"
)

// operatorName is the name of the operator being tested
const operatorName = "agent-runtime-operator"

// namespace where the project is deployed in
const namespace = "agent-runtime-operator-system"

// webhookServiceName is the name of the webhook service
const webhookServiceName = "agent-runtime-operator-webhook-service"

// webhookMutatingConfiguration is the name of the mutating webhook configuration
const webhookMutatingConfiguration = "agent-runtime-operator-mutating-webhook-configuration"

// webhookValidatingConfiguration is the name of the validating webhook configuration
const webhookValidatingConfiguration = "agent-runtime-operator-validating-webhook-configuration"

var (
	// Optional Environment Variables:
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// These variables are useful if CertManager is already installed, avoiding
	// re-installation and conflicts.
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false

	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = "example.com/agent-runtime-operator:v0.0.1"
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purposed to be used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting agent-runtime-operator integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the manager(Operator) image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

	By("loading the manager(Operator) image on Kind")
	err = utils.LoadImageToKindClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

	// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
	// To prevent errors when tests run in environments with CertManager already installed,
	// we check for its presence before execution.
	// Setup CertManager before the suite if not skipped and if not already installed
	if !skipCertManagerInstall {
		By("checking if cert manager is installed already")
		isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")
			Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
		}
	}

	// Deploy the operator
	By("creating manager namespace")
	cmd = exec.Command("kubectl", "create", "ns", namespace)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	By("installing CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the controller-manager")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	By("loading the sample images on Kind")
	// speed-up tests by pre-loading sample images used by tests from docker daemon into kind cluster
	sampleImages := []string{
		"ghcr.io/agentic-layer/weather-agent:0.3.0",
		webhookv1alpha1.DefaultTemplateImageAdk,
	}
	for _, img := range sampleImages {
		cmd = exec.Command("docker", "pull", img)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to pull image ", img)
		err = utils.LoadImageToKindClusterWithName(img)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load image ", img, " into Kind")
	}

	waitForWebhook(namespace, webhookServiceName)
	waitForWebhookCaBundleMutating(webhookMutatingConfiguration)
	waitForWebhookCaBundleValidating(webhookValidatingConfiguration)
})

var _ = AfterSuite(func() {
	By("undeploying the controller-manager")
	_, _ = utils.Run(exec.Command("make", "undeploy"))

	By("uninstalling CRDs")
	_, _ = utils.Run(exec.Command("make", "uninstall"))

	By("removing manager namespace")
	_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", namespace))

	// Teardown CertManager after the suite if not skipped and if it was not already installed
	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		utils.UninstallCertManager()
	}
})

// waitForWebhook is a helper function that waits for webhook service to be ready
func waitForWebhook(namespace string, webhookServiceName string) {
	By("waiting for webhook deployment to be ready")
	Eventually(func(g Gomega) {
		// Check that the webhook deployment is ready
		cmd := exec.Command("kubectl", "get", "deployment",
			"-l", "control-plane=controller-manager", "-n", namespace,
			"-o", "jsonpath={.items[0].status.readyReplicas}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("1"), "Controller manager should have 1 ready replica")
	}, 3*time.Minute, 5*time.Second).Should(Succeed())

	By("waiting for webhook service to be ready")
	Eventually(func(g Gomega) {
		// Check that the webhook service exists and has endpoints
		cmd := exec.Command("kubectl", "get", "service",
			webhookServiceName, "-n", namespace)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())

		// Check that the webhook service has endpoints (meaning pods are ready)
		cmd = exec.Command("kubectl", "get", "endpoints", webhookServiceName,
			"-n", namespace, "-o", "jsonpath={.subsets[*].addresses[*].ip}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).NotTo(BeEmpty(), "Webhook service should have endpoints")
	}).Should(Succeed())

	By("verifying that the certificate secret has been created")
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}).Should(Succeed())
}

func waitForWebhookCaBundle(kind string, name string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get",
			kind,
			name,
			"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
		mwhOutput, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
	}).Should(Succeed())
}

func waitForWebhookCaBundleMutating(name string) {
	waitForWebhookCaBundle(
		"mutatingwebhookconfigurations.admissionregistration.k8s.io",
		name,
	)
}
func waitForWebhookCaBundleValidating(name string) {
	waitForWebhookCaBundle(
		"validatingwebhookconfigurations.admissionregistration.k8s.io",
		name,
	)
}

func fetchKubernetesEvents() {
	By("Fetching Kubernetes events")
	cmd := exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
	eventsOutput, err := utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
	}
}

func fetchControllerManagerPodLogs() {
	By("Fetching controller manager pod logs")
	cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name="+operatorName, "-n", namespace)
	controllerLogs, err := utils.Run(cmd)
	if err == nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
	}
}
