/*
Copyright 2023.

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

// KeycloakSpec defines the desired state of Keycloak
type KeycloakSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Keycloak. Edit keycloak_types.go to remove/update
	Size               int32  `json:"size"`
	ContainerPort      int32  `json:"containerPort"`
	DbServer           string `json:"dbServer"`
	DbPort             string `json:"dbPort"`
	DbUser             string `json:"dbUser"`
	DbName             string `json:"dbName"`
	MailhogHost        string `json:"mailhogHost,omitempty"`
	MailhogPort        string `json:"mailhogPort,omitempty"`
	InitContainerImage string `json:"initContainerImage"`
	ContainerImage     string `json:"containerImage"`
}

// KeycloakStatus defines the observed state of Keycloak
type KeycloakStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Keycloak is the Schema for the keycloaks API
type Keycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeycloakSpec   `json:"spec,omitempty"`
	Status KeycloakStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeycloakList contains a list of Keycloak
type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keycloak `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Keycloak{}, &KeycloakList{})
}
