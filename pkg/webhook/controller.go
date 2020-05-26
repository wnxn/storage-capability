/*

 Copyright 2019 The KubeSphere Authors.

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

package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	pkgerrors "github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"net/http"
	"text/template"
)

const (
	annotationAddress      = "storage.kubesphere.io/storage-capability-address"
	annotationVolumeName   = "storage.kubesphere.io/storage-capability-volume-name"
	annotationMountPath    = "storage.kubesphere.io/storage-capability-mount-path"
	storageCapabilityImage = "kubespheredev/storage-capability-sidecar:v0.1.0"
	jsonContentType        = `application/json`
)

var (
	podResource           = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

// patchOperation is an operation of a JSON patch, see https://tools.ietf.org/html/rfc6902 .
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type admitFunc func(*v1.AdmissionRequest, kubernetes.Interface) ([]patchOperation, error)

// doServeAdmitFunc parese the HTTP request for an admission controller webhook
func doServeAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc, k8sClient kubernetes.Interface) ([]byte, error) {
	// Step 1: Request validation. Only handle POST request with a body and json content type.
	klog.V(4).Infof("Step 1: Request validation. Only handle POST request with a body and json content type.")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("invalid method %s, only POST requests are allowed", r.Method)
	}

	body, err := ioutil.ReadAll(r.Body)
	klog.V(4).Infof("%s", body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not read request body: %v", err)
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported content type %s, only %s is supported", contentType, jsonContentType)
	}

	// Step 2: Parse the AdmissionReview request.
	klog.V(4).Infof("Step 2: Parse the AdmissionReview request.")
	var admissionReviewReq v1.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not deserialize request: %v", err)
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, errors.New("malformed admission review: request is nil")
	}

	// Step 3: Construct the AdmissionReview response.
	klog.V(4).Infof("Step 3: Construct the AdmissionReview response.")
	admissionReviewResponse := v1.AdmissionReview{
		Response: &v1.AdmissionResponse{
			UID: admissionReviewReq.Request.UID,
		},
	}
	var patchOps []patchOperation
	patchOps, err = admit(admissionReviewReq.Request, k8sClient)
	if err != nil {
		// If the handler returned an error, incorporate the error message into the response and deny the object
		// creation.
		admissionReviewResponse.Response.Allowed = false
		admissionReviewResponse.Response.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		// Otherwise, encode the patch operations to JSON and return a positive response.
		patchBytes, err := json.Marshal(patchOps)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil, fmt.Errorf("could not marshal JSON patch: %v", err)
		}
		admissionReviewResponse.Response.Allowed = true
		admissionReviewResponse.Response.Patch = patchBytes
	}

	// Return the AdmissionReview with a response as JSON.
	bytes, err := json.Marshal(&admissionReviewResponse)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %v", err)
	}
	return bytes, nil
}

// serveAdmitFunc is a wrapper around doServeAdmitFunc that adds error handling and logging.
func serveAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc, k8sclient kubernetes.Interface) {
	klog.Info("Handling webhook request ...")
	var writeErr error
	if bytes, err := doServeAdmitFunc(w, r, admit, k8sclient); err != nil {
		klog.Info("Error handling webhook request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		klog.Info("Webhook request handled successfully")
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		klog.Info("Could not write response: %v", writeErr)
	}
}

func AdmitFuncHandler(admit admitFunc, k8sClient kubernetes.Interface) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveAdmitFunc(w, r, admit, k8sClient)
	})
}

func AddSidecarContainer(req *v1.AdmissionRequest, k8sclient kubernetes.Interface) ([]patchOperation, error) {
	if req.Resource != podResource {
		klog.Info("expect resource to be %s", podResource)
		return nil, nil
	}

	// Parse the Pod object.
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}

	klog.V(4).Infof("Handle pod %s", pod.String())
	// Retrieve labels
	addr, volName, mountPath := retrieveAnnotations(pod.GetAnnotations())
	klog.V(4).Infof("Addr: %s, VolName: %s, MountPath: %s", addr, volName, mountPath)
	// patches
	var patches []patchOperation
	if addr != "" && volName != "" && mountPath != "" {
		// patch
		// add container
		klog.V(4).Infof("Patch add containers")
		patches = append(patches, patchOperation{
			Op:   "add",
			Path: "/spec/containers/-",
			// The value must not be true if runAsUser is set to 0, as otherwise we would create a conflicting
			// configuration ourselves.
			Value: getSidecarContainerSpec(addr, volName, mountPath),
		})
	} else {
		// not patch
		return nil, nil
	}

	// Create RBAC
	klog.V(4).Infof("Create ClusterRoleBinding %s-in-%s", pod.Spec.ServiceAccountName, req.Namespace)
	if err := AddClusterRoleBinding(k8sclient, pod.Spec.ServiceAccountName, req.Namespace); err != nil {
		klog.Errorf("Add ClusterRoleBinding error: %s", err)
		return nil, err
	}
	return patches, nil
	//return nil, nil
}

func retrieveAnnotations(annotations map[string]string) (address, volumeName, mountPath string) {
	for k := range annotations {
		switch k {
		case annotationAddress:
			address = annotations[k]
		case annotationVolumeName:
			volumeName = annotations[k]
		case annotationMountPath:
			mountPath = annotations[k]
		}
	}
	return address, volumeName, mountPath
}

func getSidecarContainerSpec(addr, volName, mountPath string) corev1.Container {
	return corev1.Container{
		Args: []string{
			"--csi-address=$(ADDRESS)",
			"--v=5",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: addr,
			},
		},
		Name:            "storage-capability",
		Image:           storageCapabilityImage,
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts: []corev1.VolumeMount{
			{Name: volName, MountPath: mountPath},
		},
	}
}

func AddClusterRoleBinding(k8sclient kubernetes.Interface, sa string, ns string) error {
	// If cannot find ClusterRole, create it.
	if err := createSidecarClusterRole(k8sclient, []byte(clusterRole)); err != nil {
		return err
	}
	// Create ClusterRoleBinding
	clusterRoleBindingBytes, err := parseTemplate(clusterRoleBinding, struct {
		UniqueName              string
		ServiceAccountName      string
		ServiceAccountNamespace string
	}{
		UniqueName:              sa + "-in-" + ns,
		ServiceAccountName:      sa,
		ServiceAccountNamespace: ns,
	})
	if err != nil {
		return pkgerrors.Wrap(err, "error when parsing Sidecar ClusterRoleBinding template")
	}
	if err := createSidecarClusterRoleBinding(k8sclient, clusterRoleBindingBytes); err != nil {
		return err
	}
	return nil
}

func createSidecarClusterRole(client kubernetes.Interface, clusterRoleBytes []byte) error {
	clusterRoleEntity := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clusterRoleBytes, clusterRoleEntity); err != nil {
		return pkgerrors.Wrap(err, "unable to decode sidecar clusterrole")
	}
	if _, err := client.RbacV1().ClusterRoles().Create(clusterRoleEntity); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			klog.V(4).Infof("ClusterRole %s already exist", clusterRoleName)
			return nil
		}
		return pkgerrors.Wrapf(err, "create ClusterRole %s error", clusterRoleName)
	}
	return nil
}

func createSidecarClusterRoleBinding(client kubernetes.Interface, clusterRoleBindingBytes []byte) error {
	clusterRoleBindingEntity := &rbacv1.ClusterRoleBinding{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), clusterRoleBindingBytes, clusterRoleBindingEntity); err != nil {
		return pkgerrors.Wrap(err, "unable to decode sidecar ClusterRoleBinding")
	}
	if _, err := client.RbacV1().ClusterRoleBindings().Create(clusterRoleBindingEntity); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			klog.V(4).Infof("ClusterRoleBinding %s already exist", clusterRoleBindingEntity.GetName())
			return nil
		}
		return pkgerrors.Wrapf(err, "create ClusterRoleBinding %s error", clusterRoleBindingEntity.GetName())
	}
	return nil
}

// ParseTemplate validates and parses passed as argument template
func parseTemplate(strtmpl string, obj interface{}) ([]byte, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("template").Parse(strtmpl)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error when parsing template")
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error when executing template")
	}
	return buf.Bytes(), nil
}
