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

package sidecar

import (
	"github.com/kubesphere/storage-capability/pkg/apis/storagecapability/v1alpha1"
	clientset "github.com/kubesphere/storage-capability/pkg/generated/clientset/versioned"
	informers "github.com/kubesphere/storage-capability/pkg/generated/informers/externalversions/storagecapability/v1alpha1"
	"github.com/kubesphere/storage-capability/pkg/handler"
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
	provisionerInformer informers.Interface
	timeout             time.Duration
	resyncPeriod        time.Duration
}

func NewCSISidecarController(
	clientSet clientset.Interface,
	csiConn *grpc.ClientConn,
	timeout time.Duration,
	resyncPeriod time.Duration,
) *csiSidecarController {
	return &csiSidecarController{
		clientset:     clientSet,
		pluginHandler: handler.NewPlugin(csiConn, timeout),
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
	pcap, err := ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Get(pcapSpec.PluginInfo.Name, v1.GetOptions{})
	klog.Info(pcap)
	if err != nil {
		klog.Errorf("Get provisioner CRD error: %s", err)
	}
	if pcap.GetName() == pcapSpec.PluginInfo.Name {
		// Need to update CRD
		if !reflect.DeepEqual(pcap.Spec, pcapSpec) {
			klog.V(0).Infof("Update CRD")
			pcap.Spec = *pcapSpec
			return ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Update(pcap)
		} else {
			klog.V(0).Infof("CRD is equal to current status, nothing to update")
			return nil, nil
		}
	} else {
		// Need to create CRD
		klog.V(0).Infof("Create CRD")
		return ctrl.clientset.StorageV1alpha1().ProvisionerCapabilities().Create(
			&v1alpha1.ProvisionerCapability{
				ObjectMeta: v1.ObjectMeta{
					Name: pcapSpec.PluginInfo.Name,
				},
				Spec: *pcapSpec,
			})
	}
}
