/*

 Copyright 2020 The KubeSphere Authors.

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

package controller

import (
	"fmt"
	snapapi "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	snapinformers "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/informers/externalversions/volumesnapshot/v1beta1"
	snaplisters "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/listers/volumesnapshot/v1beta1"
	crdapi "github.com/kubesphere/storage-capability/pkg/apis/storagecapability/v1alpha1"
	clientset "github.com/kubesphere/storage-capability/pkg/generated/clientset/versioned"
	crdscheme "github.com/kubesphere/storage-capability/pkg/generated/clientset/versioned/scheme"
	crdinformers "github.com/kubesphere/storage-capability/pkg/generated/informers/externalversions/storagecapability/v1alpha1"
	crdlisters "github.com/kubesphere/storage-capability/pkg/generated/listers/storagecapability/v1alpha1"
	"k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	scinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	sclisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"time"
)

const (
	SuccessSynced = "Synced"

	ErrResourceExists = "ErrResourceExists"

	MessageResourceExists = "Resource %q already exists and is not managed by Foo"

	MessageResourceSynced = "StorageClassCapability synced successfully"

	MinimalKubernetesVersion = "v1.17.0"
)

type Controller struct {
	kubeclientset kubernetes.Interface
	crdclientset  clientset.Interface

	scLister sclisters.StorageClassLister
	scSynced cache.InformerSynced

	snapLister snaplisters.VolumeSnapshotClassLister
	snapSynced cache.InformerSynced

	pcapLister crdlisters.ProvisionerCapabilityLister
	pcapSynced cache.InformerSynced

	sccapLister crdlisters.StorageClassCapabilityLister
	sccapSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
}

// This controller is responsible to watch StorageClass, SnapshotClass, StorageClassCapability CRD and ProvisionerCapability CRD.
// And then update StorageClassCapability CRD resource object to the newest status.
func NewController(
	kubeclientset kubernetes.Interface,
	crdclientset clientset.Interface,
	scInformer scinformers.StorageClassInformer,
	snapInformer snapinformers.VolumeSnapshotClassInformer,
	pcapInformer crdinformers.ProvisionerCapabilityInformer,
	sccapInformer crdinformers.StorageClassCapabilityInformer,
) *Controller {
	utilruntime.Must(crdscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")

	controller := &Controller{
		kubeclientset: kubeclientset,
		crdclientset:  crdclientset,
		scLister:      scInformer.Lister(),
		scSynced:      scInformer.Informer().HasSynced,
		snapLister:    snapInformer.Lister(),
		snapSynced:    snapInformer.Informer().HasSynced,
		pcapLister:    pcapInformer.Lister(),
		pcapSynced:    pcapInformer.Informer().HasSynced,
		sccapLister:   sccapInformer.Lister(),
		sccapSynced:   sccapInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ProvisionerCapability"),
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	sccapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSccap,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueSccap(new)
		},
		DeleteFunc: controller.enqueueSccap,
	})
	pcapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePcap,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePcap(new)
		},
		DeleteFunc: controller.enqueuePcap,
	})
	scInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleScObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*v1.StorageClass)
			oldDepl := old.(*v1.StorageClass)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleScObject(new)
		},
		DeleteFunc: controller.handleScObject,
	})
	snapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleScObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*snapapi.VolumeSnapshotClass)
			oldDepl := old.(*snapapi.VolumeSnapshotClass)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleScObject(new)
		},
		DeleteFunc: controller.handleScObject,
	})
	return controller
}

func (c *Controller) Start(stopCh <-chan struct{}) error {
	return c.Run(5, stopCh)
}

func (c *Controller) enqueueSccap(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) enqueuePcap(obj interface{}) {
	var provisioner string
	var err error
	if provisioner, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	scList, err := c.scLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	for _, v := range scList {
		if v.Provisioner == provisioner {
			c.workqueue.Add(v.Name)
		}
	}
}

func (c *Controller) handleScObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	c.enqueueSccap(obj)
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	isValid, err := c.IsValidKubernetesVersion()
	if err != nil {
		return err
	}
	if !isValid {
		klog.V(0).Info("StorageCapability controller minimal kubernetes version %s, skipped.", MinimalKubernetesVersion)
		return nil
	}
	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting StorageClassCapability controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.scSynced, c.snapSynced, c.pcapSynced, c.sccapSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {

			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// When creating a new storage class, the controller will create a new storage capability object.
// When updating storage class, the controller will update or create the storage capability object.
// When deleting storage class, the controller will delete storage capability object.
func (c *Controller) syncHandler(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	// Get StorageClass
	sc, err := c.scLister.Get(name)
	klog.V(4).Infof("Get sc %s: entity %v", name, sc)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("storageclass '%s' in work queue no longer exists", key))
			// If StorageClass does not exist, StorageClassCapability will be deleted.
			klog.V(4).Infof("Delete StorageClassProvisioner %s", name)
			err := c.crdclientset.StorageV1alpha1().StorageClassCapabilities().Delete(name, &metav1.DeleteOptions{})
			klog.V(4).Infof("Delete StorageClassProvisioner %s, err %s", name, err.Error())
			return nil
		}
		return err
	}

	if err != nil {
		return nil
	}

	// Get StorageClass name
	sccapName := sc.GetName()
	if sccapName == "" {
		utilruntime.HandleError(fmt.Errorf("%s: storageclass name must be specified", key))
		return nil
	}
	// Get ProvisionCapability
	pcap, err := c.pcapLister.Get(sc.Provisioner)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("ProvisionerCapability %s not found", sc.Provisioner)
			return nil
		} else {
			return err
		}
	}
	// Get SnapshotClass
	snapClass, err := c.snapLister.Get(sc.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("SnapshotClass %s not found", sc.GetName())
		} else {
			return err
		}
	}
	// Get exist StorageClassCapability
	sccap, err := c.sccapLister.Get(sccapName)
	if errors.IsNotFound(err) {
		// If the resource doesn't exist, we'll create it
		klog.V(4).Infof("Create StorageClassProvisioner %s", sc.GetName())
		sccap, err = c.crdclientset.StorageV1alpha1().StorageClassCapabilities().Create(newSccap(sc, snapClass, pcap))
		return err
	}
	if err != nil {
		return err
	}
	klog.V(4).Infof("Update StorageClassProvisioner %s", sc.GetName())
	// If the resource exist, we can update it.
	_, err = c.crdclientset.StorageV1alpha1().StorageClassCapabilities().Update(updateSccap(sccap, sc, snapClass, pcap))
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) IsValidKubernetesVersion() (bool, error) {
	minVer := version.MustParseGeneric(MinimalKubernetesVersion)
	rawVer, err := c.kubeclientset.Discovery().ServerVersion()
	if err != nil {
		return false, err
	}
	ver, err := version.ParseSemantic(rawVer.String())
	if err != nil {
		return false, err
	}
	return ver.AtLeast(minVer), nil
}

func newSccap(storageClass *v1.StorageClass, snapClass *snapapi.VolumeSnapshotClass, pcap *crdapi.ProvisionerCapability) *crdapi.StorageClassCapability {
	if storageClass == nil || pcap == nil {
		return nil
	}
	res := &crdapi.StorageClassCapability{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClass.GetName(),
		},
		Spec: crdapi.StorageClassCapabilitySpec{
			Provisioner: storageClass.Provisioner,
			Features: crdapi.StorageClassCapabilitySpecFeatures{
				Topology: pcap.Spec.Features.Topology,
				Volume:   pcap.Spec.Features.Volume,
			},
		},
	}
	// set volume features
	if *storageClass.AllowVolumeExpansion != true {
		res.Spec.Features.Volume.Expand = crdapi.ExpandModeUnknown
	}
	// set snapshot features
	if snapClass != nil && snapClass.Driver == pcap.GetName() {
		res.Spec.Features.Snapshot = pcap.Spec.Features.Snapshot
	}
	klog.V(4).Info("Create: ", res)
	return res
}

func updateSccap(sccap *crdapi.StorageClassCapability, storageClass *v1.StorageClass, snapClass *snapapi.VolumeSnapshotClass, pcap *crdapi.ProvisionerCapability) *crdapi.StorageClassCapability {
	if sccap == nil || storageClass == nil || pcap == nil {
		return nil
	}
	if sccap.GetName() != storageClass.GetName() {
		klog.Errorf("StorageClassCapability name should be the same as StorageClass name, but %s != %s", sccap.GetName(), storageClass.GetName())
		return nil
	}
	res := sccap.DeepCopy()
	res.Spec = crdapi.StorageClassCapabilitySpec{
		Provisioner: storageClass.Provisioner,
		Features: crdapi.StorageClassCapabilitySpecFeatures{
			Topology: pcap.Spec.Features.Topology,
			Volume:   pcap.Spec.Features.Volume,
		},
	}
	// set volume features
	if *storageClass.AllowVolumeExpansion != true {
		res.Spec.Features.Volume.Expand = crdapi.ExpandModeUnknown
	}
	// set snapshot features
	if snapClass != nil && snapClass.GetName() == storageClass.GetName() && snapClass.Driver == pcap.GetName() {
		res.Spec.Features.Snapshot = pcap.Spec.Features.Snapshot
	}
	klog.V(4).Info("Update: ", res)
	return res
}
