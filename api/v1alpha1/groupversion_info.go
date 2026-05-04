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

// Package v1alpha1 contains API Schema definitions for the runtime v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=runtime.agentic-layer.ai
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "runtime.agentic-layer.ai", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &schemeBuilder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// schemeBuilder mirrors the deprecated sigs.k8s.io/controller-runtime/pkg/scheme.Builder
// but only depends on k8s.io/apimachinery, keeping the api package import-light.
type schemeBuilder struct {
	runtime.SchemeBuilder
	GroupVersion schema.GroupVersion
}

func (bld *schemeBuilder) Register(objects ...runtime.Object) *schemeBuilder {
	bld.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(bld.GroupVersion, objects...)
		metav1.AddToGroupVersion(s, bld.GroupVersion)
		return nil
	})
	return bld
}
