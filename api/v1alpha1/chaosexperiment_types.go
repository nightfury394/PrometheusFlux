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

// ChaosExperimentSpec defines the desired state of ChaosExperiment
type ChaosExperimentSpec struct {
	// Target defines the selection criteria for the chaos experiment.
	Target ExperimentTarget `json:"target"`

	// Attack defines the type of chaos attack to perform.
	Attack ExperimentAttack `json:"attack"`

	// Duration specifies how long the experiment should run.
	// This is a string representation of a Go duration (e.g., "30s", "5m").
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Mode specifies the execution mode of the experiment: "one-shot" or "recurring".
	// Defaults to "one-shot".
	// +kubebuilder:default="one-shot"
	// +kubebuilder:validation:Enum=one-shot;recurring
	// +optional
	Mode ExperimentMode `json:"mode,omitempty"`
}

// ExperimentTarget defines the target for the chaos experiment.
type ExperimentTarget struct {
	// Namespace is the target Kubernetes namespace.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// LabelSelector is a map of key-value pairs used to select target pods.
	// +kubebuilder:validation:MinProperties=1
	LabelSelector map[string]string `json:"labelSelector"`
}

// ExperimentAttack defines the type of attack.
type ExperimentAttack struct {
	// Type of attack to perform. Currently, only "pod-kill" is supported.
	// +kubebuilder:validation:Enum=pod-kill
	Type AttackType `json:"type"`
}

// AttackType represents the type of chaos attack.
type AttackType string

const (
	// PodKillAttack represents the pod-kill chaos attack.
	PodKillAttack AttackType = "pod-kill"
)

// ExperimentMode represents the execution mode of the experiment.
type ExperimentMode string

const (
	// OneShotMode means the experiment runs once and then completes.
	OneShotMode ExperimentMode = "one-shot"
	// RecurringMode means the experiment runs repeatedly based on its duration.
	RecurringMode ExperimentMode = "recurring"
)

// ChaosExperimentStatus defines the observed state of ChaosExperiment.
type ChaosExperimentStatus struct {
	// Phase indicates the current state of the chaos experiment.
	// Possible values are "Pending", "Running", "Completed", "Failed".
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	// +optional
	Phase ExperimentPhase `json:"phase,omitempty"`

	// LastRunTime records the last time the experiment performed an action.
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// Message provides a human-readable status or error message.
	// +optional
	Message string `json:"message,omitempty"`

	// conditions represent the current state of the ChaosExperiment resource.
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

// ExperimentPhase represents the current phase of the chaos experiment.
type ExperimentPhase string

const (
	// ExperimentPending indicates the experiment is waiting to start.
	ExperimentPending ExperimentPhase = "Pending"
	// ExperimentRunning indicates the experiment is currently active.
	ExperimentRunning ExperimentPhase = "Running"
	// ExperimentCompleted indicates the experiment has finished successfully.
	ExperimentCompleted ExperimentPhase = "Completed"
	// ExperimentFailed indicates the experiment encountered an unrecoverable error.
	ExperimentFailed ExperimentPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ChaosExperiment is the Schema for the chaosexperiments API
type ChaosExperiment struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ChaosExperiment
	// +required
	Spec ChaosExperimentSpec `json:"spec"`

	// status defines the observed state of ChaosExperiment
	// +optional
	Status ChaosExperimentStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ChaosExperimentList contains a list of ChaosExperiment
type ChaosExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ChaosExperiment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChaosExperiment{}, &ChaosExperimentList{})
}
