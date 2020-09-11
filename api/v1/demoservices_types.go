/*


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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DemoServicesSpec defines the desired state of DemoServices
type DemoServicesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of DemoServices. Edit DemoServices_types.go to remove/update
	Services []Service `json:"services"`
}

// Service defines the desired state of a Service
type Service struct {
	Name     string    `json:"name"`
	Image    string    `json:"image"`
	Version  string    `json:"version"`
	Replicas int32     `json:"replicas"`
	Limits   Resources `json:"limits"`
	Requests Resources `json:"requests"`
}

// Resources defines the desired resources for limits and requests
type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// DemoServicesStatus defines the observed state of DemoServices
type DemoServicesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name   string `json:"name"`
	Status string `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DemoServices is the Schema for the demoservices API
type DemoServices struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DemoServicesSpec   `json:"spec,omitempty"`
	Status DemoServicesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DemoServicesList contains a list of DemoServices
type DemoServicesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DemoServices `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DemoServices{}, &DemoServicesList{})
}
