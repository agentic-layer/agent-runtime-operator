// internal/equality/equality_test.go

package equality_test

import (
	"testing"

	"github.com/agentic-layer/agent-runtime-operator/internal/equality"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// A helper function to easily get a pointer to a bool, as required by the 'Optional' field.
func boolPtr(b bool) *bool {
	return &b
}

func TestEnvVarsEqual(t *testing.T) {
	// Define a common EnvVar with a ValueFrom for deep comparison
	secretKeyRef := &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
		Key:                  "api-key",
	}

	testCases := []struct {
		name string
		a    []corev1.EnvVar
		b    []corev1.EnvVar
		want bool
	}{
		{
			name: "should be equal for identical slices",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			want: true,
		},
		{
			name: "should be equal for slices in different order",
			a: []corev1.EnvVar{
				{Name: "VAR1", Value: "VAL1"},
				{Name: "VAR2", Value: "VAL2"},
			},
			b: []corev1.EnvVar{
				{Name: "VAR2", Value: "VAL2"},
				{Name: "VAR1", Value: "VAL1"},
			},
			want: true,
		},
		{
			name: "should be equal for empty slices",
			a:    []corev1.EnvVar{},
			b:    []corev1.EnvVar{},
			want: true,
		},
		{
			name: "should be equal for nil slices",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "should be equal with ValueFrom fields",
			a:    []corev1.EnvVar{{Name: "API_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: secretKeyRef}}},
			b:    []corev1.EnvVar{{Name: "API_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: secretKeyRef}}},
			want: true,
		},
		{
			name: "should not be equal for different lengths",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}, {Name: "VAR2", Value: "VAL2"}},
			want: false,
		},
		{
			name: "should not be equal for different values",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "VAR1", Value: "DIFFERENT"}},
			want: false,
		},
		{
			name: "should not be equal for different ValueFrom fields",
			a:    []corev1.EnvVar{{Name: "API_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: secretKeyRef}}},
			b:    []corev1.EnvVar{{Name: "API_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "different-key"}}}},
			want: false,
		},
		{
			name: "should not be equal for nil vs empty slice",
			a:    nil,
			b:    []corev1.EnvVar{},
			want: false,
		},
		{
			name: "should not be equal for populated vs nil",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    nil,
			want: false,
		},
		{
			name: "should not be equal for different names",
			a:    []corev1.EnvVar{{Name: "VAR_A", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "VAR_B", Value: "VAL1"}},
			want: false,
		},
		// Tests for edge cases that should be blocked by API server validation.
		{
			name: "should not be equal if one var has an empty name",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "", Value: "VAL1"}},
			want: false,
		},
		{
			name: "should be equal if values are both empty",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: ""}},
			b:    []corev1.EnvVar{{Name: "VAR1", Value: ""}},
			want: true,
		},
		{
			name: "should not be equal for value vs empty value",
			a:    []corev1.EnvVar{{Name: "VAR1", Value: "VAL1"}},
			b:    []corev1.EnvVar{{Name: "VAR1", Value: ""}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := equality.EnvVarsEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("EnvVarsEqual() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnvFromEqual(t *testing.T) {
	testCases := []struct {
		name string
		a    []corev1.EnvFromSource
		b    []corev1.EnvFromSource
		want bool
	}{
		{
			name: "should be equal for identical slices",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			want: true,
		},
		{
			name: "should be equal for slices in different order",
			a: []corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}},
			},
			b: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "secret1"}}},
				{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}},
			},
			want: true,
		},
		{
			name: "should not be equal for different lengths",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    []corev1.EnvFromSource{},
			want: false,
		},
		{
			name: "should not be equal for different configmap names",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "different-cm"}}}},
			want: false,
		},
		{
			name: "should not be equal if one source is invalid (empty name)",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: ""}}}},
			want: false,
		},
		{
			name: "should be equal for two nil slices",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "should not be equal for nil vs empty slice",
			a:    nil,
			b:    []corev1.EnvFromSource{},
			want: false,
		},
		{
			name: "should not be equal for populated vs nil",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    nil,
			want: false,
		},
		{
			name: "should not be equal for different types with same name",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-resource"}}}},
			b:    []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-resource"}}}},
			want: false,
		},
		{
			name: "should not be equal for different prefixes",
			a:    []corev1.EnvFromSource{{Prefix: "P1_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			b:    []corev1.EnvFromSource{{Prefix: "P2_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}}}},
			want: false,
		},
		{
			name: "should not be equal for different optional flags",
			a:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}, Optional: boolPtr(true)}}},
			b:    []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"}, Optional: boolPtr(false)}}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := equality.EnvFromEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("EnvFromEqual() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProbesEqual(t *testing.T) {
	testCases := []struct {
		name string
		a    *corev1.Probe
		b    *corev1.Probe
		want bool
	}{
		{
			name: "should be equal for both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "should not be equal for one nil",
			a:    nil,
			b:    &corev1.Probe{InitialDelaySeconds: 10},
			want: false,
		},
		{
			name: "should not be equal for other nil",
			a:    &corev1.Probe{InitialDelaySeconds: 10},
			b:    nil,
			want: false,
		},
		{
			name: "should be equal for identical HTTP probes",
			a: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(8000),
					},
				},
				InitialDelaySeconds: 10,
				PeriodSeconds:       5,
			},
			b: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(8000),
					},
				},
				InitialDelaySeconds: 10,
				PeriodSeconds:       5,
			},
			want: true,
		},
		{
			name: "should be equal for identical TCP probes",
			a: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8000),
					},
				},
				InitialDelaySeconds: 10,
			},
			b: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8000),
					},
				},
				InitialDelaySeconds: 10,
			},
			want: true,
		},
		{
			name: "should not be equal for different HTTP paths",
			a: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(8000),
					},
				},
			},
			b: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(8000),
					},
				},
			},
			want: false,
		},
		{
			name: "should not be equal for different ports",
			a: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8000),
					},
				},
			},
			b: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(3000),
					},
				},
			},
			want: false,
		},
		{
			name: "should not be equal for different probe types",
			a: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(8000),
					},
				},
			},
			b: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8000),
					},
				},
			},
			want: false,
		},
		{
			name: "should not be equal for different timing settings",
			a: &corev1.Probe{
				InitialDelaySeconds: 10,
			},
			b: &corev1.Probe{
				InitialDelaySeconds: 15,
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := equality.ProbesEqual(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("ProbesEqual() = %v, want %v", got, tc.want)
			}
		})
	}
}
