/*
Copyright 2024 tanuudev.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DevenvSize string

const (
	SizeSmall  DevenvSize = "small"
	SizeMedium DevenvSize = "medium"
	SizeLarge  DevenvSize = "large"
)

type NodeInfo struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	CreatedAt string `json:"createdAt"`
}

// DevenvSpec defines the desired state of Devenv
type DevenvSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Devenv. Edit devenv_types.go to remove/update
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	Name string `json:"name"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	CloudProvider string `json:"cloudProvider"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	K8sVersion string `json:"k8sVersion"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	WorkerSelector string `json:"workerSelector,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	CtrlSelector string `json:"ctrlSelector,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	StorageSelector string `json:"storageSelector,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	GpuSelector string `json:"gpuSelector,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	Zone string `json:"zone,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	TemplateCM string `json:"templateCM,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	ProviderConfig string `json:"providerConfig,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	CtrlMachineType string `json:"ctrlMachineType,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	WorkerMachineType string `json:"workerMachineType,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	GpuMachineType string `json:"gpuMachineType,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	Subnetwork string `json:"subnetwork,omitempty"`
	// +kubebuilder:validation:X-Kubernetes-Immutable=true
	Size DevenvSize `json:"size,omitempty"`

	TalosVersion   string `json:"talosVersion"`
	WorkerReplicas int    `json:"workerReplicas"`
	CtrlReplicas   int    `json:"ctrlReplicas,omitempty"`
	GpuReplicas    int    `json:"gpuReplicas"`
}

// DevenvStatus defines the observed state of Devenv
type DevenvStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ControlPlane   []NodeInfo `json:"controlPlane,omitempty"`
	Workers        []NodeInfo `json:"workers,omitempty"`
	Gpus           []NodeInfo `json:"gpus,omitempty"`
	IpAddress      string     `json:"ipAddress,omitempty"`
	CloudProvider  string     `json:"cloudProvider,omitempty"`
	Status         string     `json:"status,omitempty"`
	Kubeconfig     string     `json:"kubeconfig,omitempty"`
	Services       []string   `json:"services,omitempty"`
	WorkerReplicas int        `json:"workerReplicas,omitempty"`
	GpuReplicas    int        `json:"gpuReplicas,omitempty"`
	WorkerSelector string     `json:"workerSelector,omitempty"`
	CtrlSelector   string     `json:"ctrlSelector,omitempty"`
	GpuSelector    string     `json:"gpuSelector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="WorkerCount",type=string,JSONPath=`.status.workerReplicas`
// +kubebuilder:printcolumn:name="GPUCount",type=string,JSONPath=`.status.gpuReplicas`

// Devenv is the Schema for the devenvs API
type Devenv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DevenvSpec   `json:"spec,omitempty"`
	Status DevenvStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DevenvList contains a list of Devenv
type DevenvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Devenv `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Devenv{}, &DevenvList{})
}
