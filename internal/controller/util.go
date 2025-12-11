package controller

import (
	corev1 "k8s.io/api/core/v1"
)

// findEnvVar finds an environment variable by name in a slice of EnvVars
func findEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			return &envVars[i]
		}
	}
	return nil
}

func findContainerByName(pod *corev1.PodSpec, name string) *corev1.Container {
	for i := range pod.Containers {
		if pod.Containers[i].Name == name {
			return &pod.Containers[i]
		}
	}
	return nil
}

// getNamespaceWithDefault returns the namespace from an ObjectReference,
// defaulting to the provided default namespace if not specified.
// This is useful for resources that can reference objects in the same or different namespaces.
func getNamespaceWithDefault(objRef *corev1.ObjectReference, defaultNamespace string) string {
	if objRef.Namespace != "" {
		return objRef.Namespace
	}
	return defaultNamespace
}
