package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Backend struct {
	Host 						string 								`json:"host"`
	Port 						int 									`json:"port"`
	Credentials 		v1.SecretReference		`json:"credentials"`
}

type Database struct {
	CharacterSet 		string 								`json:"characterSet"`
	Collate 				string 								`json:"collate"`
	Credentials 		v1.SecretReference 		`json:"credentials"`
	Privileges  		[]string							`json:"privileges"`
	Host						string								`json:"host"`
}

// MysqlSpec defines the desired state of Mysql
type MysqlSpec struct {
	Backend 				Backend				        `json:"backend"`
	Database				Database							`json:"database"`
}

// MysqlStatus defines the observed state of Mysql
type MysqlStatus struct {
	Stage    		 			string   `json:"stage"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Mysql is the Schema for the mysqls API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mysqls,scope=Namespaced
type Mysql struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlSpec   `json:"spec,omitempty"`
	Status MysqlStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlList contains a list of Mysql
type MysqlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mysql `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mysql{}, &MysqlList{})
}
