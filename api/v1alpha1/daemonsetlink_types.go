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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceRef defines the reference to the monitored Workload (Deployment or StatefulSet)
type SourceRef struct {
	// API version of the referent.
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`
	// Kind of the referent - must be Deployment or StatefulSet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Deployment;StatefulSet
	Kind string `json:"kind"`
	// Name of the referent.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the referent.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// TargetDaemonSetRef defines the reference to the target DaemonSet
type TargetDaemonSetRef struct {
	// Name of the target DaemonSet.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the target DaemonSet.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// DaemonSetLinkSpec defines the desired state of DaemonSetLink
type DaemonSetLinkSpec struct {
	// SourceRef defines the reference to the monitored Workload (Deployment or StatefulSet)
	// +kubebuilder:validation:Required
	SourceRef SourceRef `json:"sourceRef"`
	// TargetDaemonSetRef defines the reference to the target DaemonSet
	// +kubebuilder:validation:Required
	TargetDaemonSetRef TargetDaemonSetRef `json:"targetDaemonSetRef"`
	// DisabledNodeSelector specifies a non-existent node selector to disable the DaemonSet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	DisabledNodeSelector map[string]string `json:"disabledNodeSelector"`
}

// DaemonSetLinkStatus defines the observed state of DaemonSetLink
type DaemonSetLinkStatus struct {
	// Linked indicates if the DaemonSet is linked to the Workload
	// +optional
	Linked bool `json:"linked,omitempty"`
	// Message provides additional information about the status
	// +optional
	Message string `json:"message,omitempty"`
	// ObservedGeneration is the last generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastUpdateTime is the last time the status was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Represents the latest available observations of a DaemonSetLink's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DaemonSetLink is the Schema for the daemonsetlinks API
type DaemonSetLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DaemonSetLinkSpec   `json:"spec,omitempty"`
	Status DaemonSetLinkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DaemonSetLinkList contains a list of DaemonSetLink
type DaemonSetLinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DaemonSetLink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DaemonSetLink{}, &DaemonSetLinkList{})
}
