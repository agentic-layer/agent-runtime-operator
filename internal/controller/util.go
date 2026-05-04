package controller

import (
	"maps"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	appLabel           = "app"
	conditionTypeReady = "Ready"
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

// applyCommonMetadataToObjectMeta merges commonMetadata labels and annotations into the
// provided ObjectMeta. Managed labels should be applied after this call so they take precedence.
func applyCommonMetadataToObjectMeta(obj *metav1.ObjectMeta, commonMetadata *runtimev1alpha1.EmbeddedMetadata) {
	if commonMetadata == nil {
		return
	}
	if len(commonMetadata.Labels) > 0 {
		if obj.Labels == nil {
			obj.Labels = make(map[string]string)
		}
		maps.Copy(obj.Labels, commonMetadata.Labels)
	}
	if len(commonMetadata.Annotations) > 0 {
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		maps.Copy(obj.Annotations, commonMetadata.Annotations)
	}
}

// buildPodTemplateMetadata builds the labels and annotations for a pod template.
// commonMetadata and podMetadata are merged, with podMetadata taking precedence.
// selectorLabels are always enforced last to ensure the pod selector is never overridden.
// Returns nil annotations if no annotations are configured.
func buildPodTemplateMetadata(
	selectorLabels map[string]string,
	commonMetadata, podMetadata *runtimev1alpha1.EmbeddedMetadata,
) (labels, annotations map[string]string) {
	podLabels := map[string]string{}
	if commonMetadata != nil {
		maps.Copy(podLabels, commonMetadata.Labels)
	}
	if podMetadata != nil {
		maps.Copy(podLabels, podMetadata.Labels)
	}
	// Selector labels always take precedence
	maps.Copy(podLabels, selectorLabels)

	podAnnotations := map[string]string{}
	if commonMetadata != nil {
		maps.Copy(podAnnotations, commonMetadata.Annotations)
	}
	if podMetadata != nil {
		maps.Copy(podAnnotations, podMetadata.Annotations)
	}

	if len(podAnnotations) == 0 {
		return podLabels, nil
	}
	return podLabels, podAnnotations
}

// findReadyCondition returns the "Ready" condition from the given conditions slice, or nil if not found.
func findReadyCondition(conditions []metav1.Condition) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionTypeReady {
			return &conditions[i]
		}
	}
	return nil
}
