package equality

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

// EnvVarsEqual compares environment variable slices for equality, ignoring order.
func EnvVarsEqual(existing, desired []corev1.EnvVar) bool {
	// Sort slices by the 'Name' field before comparing.
	sortFunc := func(a, b corev1.EnvVar) bool {
		return a.Name < b.Name
	}
	return cmp.Equal(existing, desired, cmpopts.SortSlices(sortFunc))
}

// EnvFromEqual compares environment variable source slices for equality, ignoring order.
func EnvFromEqual(existing, desired []corev1.EnvFromSource) bool {
	// Sort slices by their generated key before comparing.
	sortFunc := func(a, b corev1.EnvFromSource) bool {
		return getEnvFromSourceKey(a) < getEnvFromSourceKey(b)
	}
	return cmp.Equal(existing, desired, cmpopts.SortSlices(sortFunc))
}

// getEnvFromSourceKey generates a unique key for an EnvFromSource.
func getEnvFromSourceKey(s corev1.EnvFromSource) string {
	if s.ConfigMapRef != nil {
		return fmt.Sprintf("configmap:%s:%s:%t", s.Prefix, s.ConfigMapRef.Name, s.ConfigMapRef.Optional != nil && *s.ConfigMapRef.Optional)
	}
	if s.SecretRef != nil {
		return fmt.Sprintf("secret:%s:%s:%t", s.Prefix, s.SecretRef.Name, s.SecretRef.Optional != nil && *s.SecretRef.Optional)
	}
	return ""
}
