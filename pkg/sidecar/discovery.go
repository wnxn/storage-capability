package sidecar

import (
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
)

const (
	SupportedMinimalKubernetesVersion = "v1.17.0"
)

func IsValidKubernetesVersion(clientset kubernetes.Clientset, minVer version.Version) (bool, error) {
	rawVer, err := clientset.ServerVersion()
	if err != nil {
		return false, err
	}
	ver, err := version.ParseSemantic(rawVer.String())
	if err != nil {
		return false, err
	}
	return ver.AtLeast(&minVer), nil
}
