/*
Copyright 2024 punasusi.

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

// DevenvSpec defines the desired state of Devenv
type DevenvSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Devenv. Edit devenv_types.go to remove/update
	Name          string `json:"name,omitempty"`
	CloudProvider string `json:"cloudProvider,omitempty"`
	Gpu           bool   `json:"gpu,omitempty"`
}

// DevenvStatus defines the observed state of Devenv
type DevenvStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ControlPlane  []string `json:"controlPlane,omitempty"`
	Workers       []string `json:"workers,omitempty"`
	Gpus          []string `json:"gpus,omitempty"`
	IpAddress     string   `json:"ipAddress,omitempty"`
	CloudProvider string   `json:"cloudProvider,omitempty"`
	Status        string   `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
