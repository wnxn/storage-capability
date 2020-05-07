package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProvisionerCapability struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProvisionerCapabilitySpec `json:"spec"`
}

type ProvisionerCapabilitySpec struct {
	PluginInfo ProvisionerCapabilitySpecPluginInfo `json:"pluginInfo"`
	Features   ProvisionerCapabilitySpecFeatures   `json:"features"`
}

type ProvisionerCapabilitySpecPluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ProvisionerCapabilitySpecFeatures struct {
	Topology bool                                      `json:"topology"`
	Volume   ProvisionerCapabilitySpecFeaturesVolume   `json:"volume"`
	Snapshot ProvisionerCapabilitySpecFeaturesSnapshot `json:"snapshot"`
}

type ProvisionerCapabilitySpecFeaturesVolume struct {
	Create bool       `json:"create"`
	Attach bool       `json:"attach"`
	List   bool       `json:"list"`
	Clone  bool       `json:"clone"`
	Stats  bool       `json:"stats"`
	Expand ExpandMode `json:"expandMode"`
}

type ProvisionerCapabilitySpecFeaturesSnapshot struct {
	Create bool `json:"create"`
	List   bool `json:"list"`
}

type ExpandMode string

const (
	ExpandModeUnknown ExpandMode = "UNKNOWN"
	ExpandModeOffline ExpandMode = "OFFLINE"
	ExpandModeOnline  ExpandMode = "ONLINE"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProvisionerCapabilityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProvisionerCapability `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StorageClassCapability struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StorageClassCapabilitySpec `json:"spec"`
}

type StorageClassCapabilitySpec struct {
	Provisioner string                             `json:"provisioner"`
	Features    StorageClassCapabilitySpecFeatures `json:"features"`
}

type StorageClassCapabilitySpecFeatures struct {
	Topology bool                                      `json:"topology"`
	Volume   ProvisionerCapabilitySpecFeaturesVolume   `json:"volume"`
	Snapshot ProvisionerCapabilitySpecFeaturesSnapshot `json:"snapshot"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StorageClassCapabilityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []StorageClassCapability `json:"items"`
}
