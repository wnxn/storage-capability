package sidecar

import (
	"context"
	"github.com/wnxn/storage-capability/pkg/apis/storagecapability/v1alpha1"
	clientset "github.com/wnxn/storage-capability/pkg/generated/clientset/versioned"
	informers "github.com/wnxn/storage-capability/pkg/generated/informers/externalversions/storagecapability/v1alpha1"
	"github.com/wnxn/storage-capability/pkg/handler"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"reflect"
	"time"
)

type csiSidecarController struct {
	clientset           clientset.Interface
	pluginHandler       handler.PluginHandler
	driverName          string
	provisionerInformer informers.Interface
	timeout             time.Duration
	resyncPeriod        time.Duration
}

func NewCSISidecarController(
	clientSet clientset.Interface,
	csiConn *grpc.ClientConn,
	driverName string,
	timeout time.Duration,
	resyncPeriod time.Duration,
) *csiSidecarController {
	return &csiSidecarController{
		clientset:     clientSet,
		pluginHandler: handler.NewPlugin(csiConn, timeout),
		driverName:    driverName,
		timeout:       timeout,
		resyncPeriod:  resyncPeriod,
	}
}

func (ctrl *csiSidecarController) Run(stopCh <-chan struct{}) {
	klog.V(0).Info("Starting sidecar controller")
	defer klog.V(0).Info("Shutting sidecar controller")
	go wait.Until(ctrl.contentWorker, ctrl.resyncPeriod, stopCh)
	<-stopCh
}

func (ctrl *csiSidecarController) contentWorker() {
	// Get Capability from plugin
	pcapSpec, err := ctrl.pluginHandler.GetFullCapability()
	if err != nil {
		return
	}
	// Check capability
	if pcapSpec.PluginInfo.Name != ctrl.driverName {
		klog.Errorf("Provisioner name mismatch error: expect %s, but actually %s", ctrl.driverName, pcapSpec.PluginInfo.Name)
		return
	}
	// Create or update Provisioner CRD
	pcap, err := ctrl.createOrUpdateProvisionerCRD(pcapSpec)
	if err != nil {
		klog.Errorf("Create or update provisioner CRD error: %s", err)
		return
	}
	klog.V(5).Infof("Succeed to create or update CRD %v", pcap)
}

func (ctrl *csiSidecarController) createOrUpdateProvisionerCRD(pcapSpec *v1alpha1.ProvisionerCapabilitySpec) (*v1alpha1.ProvisionerCapability, error) {
	if pcapSpec == nil {
		klog.Warning("Update nothing")
		return nil, nil
	}
	// Check object existed
	ctx, cancel := context.WithTimeout(context.Background(), ctrl.timeout)
	defer cancel()
	pcap, err := ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Get(ctx, ctrl.driverName, v1.GetOptions{})
	klog.Info(pcap)
	if err != nil {
		klog.Errorf("Get provisioner CRD error: %s", err)
	}
	if pcap.GetName() == ctrl.driverName {
		// Need to update CRD
		if !reflect.DeepEqual(pcap.Spec, pcapSpec) {
			klog.V(0).Infof("Update CRD")
			pcap.Spec = *pcapSpec
			ctx, cancel := context.WithTimeout(context.Background(), ctrl.timeout)
			defer cancel()
			return ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Update(ctx, pcap, v1.UpdateOptions{})
		} else {
			klog.V(0).Infof("CRD is equal to current status, nothing to update")
			return nil, nil
		}
	} else {
		// Need to create CRD
		klog.V(0).Infof("Create CRD")
		ctx, cancel := context.WithTimeout(context.Background(), ctrl.timeout)
		defer cancel()
		return ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Create(ctx,
			&v1alpha1.ProvisionerCapability{
				ObjectMeta: v1.ObjectMeta{
					Name: ctrl.driverName,
				},
				Spec: *pcapSpec,
			}, v1.CreateOptions{})
	}
}
