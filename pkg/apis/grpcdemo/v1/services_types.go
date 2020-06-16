package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServicesSpec defines the desired state of Services
type ServicesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
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

// ServiceStatus defines the observed state of a single Service
type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ServicesStatus defines the observed state of Services
type ServicesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Services []ServiceStatus `json:"services"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Services is the Schema for the services API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=services,scope=Namespaced
type Services struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServicesSpec   `json:"spec,omitempty"`
	Status ServicesStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServicesList contains a list of Services
type ServicesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Services `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Services{}, &ServicesList{})
}
