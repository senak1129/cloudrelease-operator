/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CloudReleaseSpec defines the desired state of CloudRelease.
type CloudReleaseSpec struct {
	// image is the container image deployed by this release.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Image string `json:"image"`

	// replicas is the desired number of application Pods.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// port is the container port exposed by the generated Service.
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

// CloudReleaseStatus defines the observed state of CloudRelease.
type CloudReleaseStatus struct {
	// observedGeneration is the most recent generation processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// readyReplicas is the number of Pods that currently pass readiness checks.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// conditions represent the current state of the CloudRelease resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=crls,categories=all
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// CloudRelease is the Schema for the cloudreleases API.
type CloudRelease struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CloudRelease
	// +required
	Spec CloudReleaseSpec `json:"spec"`

	// status defines the observed state of CloudRelease
	// +optional
	Status CloudReleaseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CloudReleaseList contains a list of CloudRelease.
type CloudReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CloudRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &CloudRelease{}, &CloudReleaseList{})
		return nil
	})
}
