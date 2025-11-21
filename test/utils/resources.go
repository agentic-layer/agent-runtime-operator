package utils

import (
	"fmt"
	"os/exec"
	"time"
)

// VerifyDeploymentReady verifies that a deployment is ready within the given timeout
func VerifyDeploymentReady(name, namespace string, timeout time.Duration) error {
	cmd := exec.Command("kubectl", "wait", "deployment", name, "-n", namespace,
		"--for=condition=Available", "--timeout="+timeout.String())
	if output, err := Run(cmd); err != nil {
		describeDeployment, _ := Run(exec.Command("kubectl", "describe", "deployment", name, "-n", namespace))
		describePods, _ := Run(exec.Command("kubectl", "describe", "pod", "-l", "app="+name, "-n", namespace))
		return fmt.Errorf("deployment is not ready (%s):\n%s\nPods:\n%s",
			output, describeDeployment, describePods,
		)
	}
	return nil
}

// DeleteAgent deletes an agent resource
func DeleteAgent(name, namespace string) error {
	cmd := exec.Command("kubectl", "delete", "agent", name, "-n", namespace)
	if output, err := Run(cmd); err != nil {
		return fmt.Errorf("failed to delete agent %s in namespace %s: %s", name, namespace, output)
	}
	return nil
}
