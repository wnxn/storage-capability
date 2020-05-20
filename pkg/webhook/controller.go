package webhook

import (
	"fmt"
	"k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
	"net/http"
)

const (
	AnnotationAddress      = "storage-capability-address"
	AnnotationVolumeName   = "storage-capability-volume-name"
	AnnotationMountPath    = "storage-capability-mount-path"
	StorageCapabilityImage = "wangxinsh/storage-capability-sidecar:v0.1.0"
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

type admitFunc func(*v1.AdmissionRequest) ([]patchOperation, error)

func doServeAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) ([]byte, error) {
	return nil, nil
}

// serveAdmitFunc is a wrapper around doServeAdmitFunc that adds error handling and logging.
func serveAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	klog.Info("Handling webhook request ...")

	var writeErr error
	if bytes, err := doServeAdmitFunc(w, r, admit); err != nil {
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

func AdmitFuncHandler(admit admitFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveAdmitFunc(w, r, admit)
	})
}

func ApplySecurityDefaults(req *v1.AdmissionRequest) ([]patchOperation, error) {
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

	// Retrieve labels
	addr, volName, mountPath := RetrieveAnnotations(pod.GetAnnotations())
	if addr != "" {

	}

	// patches
	var patches []patchOperation
	if addr != "" && volName != "" && mountPath != "" {
		// patch
		// add container
		patches = append(patches, patchOperation{
			Op:   "add",
			Path: "/spec/containers",
			// The value must not be true if runAsUser is set to 0, as otherwise we would create a conflicting
			// configuration ourselves.
			Value: GetSidecarContainerSpec(addr, volName, mountPath),
		})
	} else {
		// not patch
		return nil, nil
	}
	return patches, nil
}

func RetrieveAnnotations(labels map[string]string) (address, volumeName, mountPath string) {
	for k := range labels {
		switch k {
		case AnnotationAddress:
			address = labels[k]
		case AnnotationVolumeName:
			volumeName = labels[k]
		case AnnotationMountPath:
			mountPath = labels[k]
		}
	}
	return address, volumeName, mountPath
}

func GetSidecarContainerSpec(addr, volName, mountPath string) corev1.Container {
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
		Image:           StorageCapabilityImage,
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts: []corev1.VolumeMount{
			{Name: volName, MountPath: mountPath},
		},
	}
}
