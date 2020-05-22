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
	snapbeta1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	snapfake "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned/fake"
	snapinformers "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/informers/externalversions"
	crdv1alpha1 "github.com/kubesphere/storage-capability/pkg/apis/storagecapability/v1alpha1"
	crdfake "github.com/kubesphere/storage-capability/pkg/generated/clientset/versioned/fake"
	crdinformers "github.com/kubesphere/storage-capability/pkg/generated/informers/externalversions"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"testing"
	"time"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	crdclient  *crdfake.Clientset
	kubeclient *k8sfake.Clientset
	snapclient *snapfake.Clientset

	sccapLister []*crdv1alpha1.StorageClassCapability
	pcapLister  []*crdv1alpha1.ProvisionerCapability
	scLister    []*storagev1.StorageClass
	snapLister  []*snapbeta1.VolumeSnapshotClass

	kubeactions []core.Action
	crdaction   []core.Action
	snapaction  []core.Action

	kubeobject []runtime.Object
	crdobject  []runtime.Object
	snapobject []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.kubeobject = []runtime.Object{}
	f.crdobject = []runtime.Object{}
	f.snapobject = []runtime.Object{}
	return f
}

func newProvisionerCapability(provisioner string) *crdv1alpha1.ProvisionerCapability {
	return &crdv1alpha1.ProvisionerCapability{
		ObjectMeta: v1.ObjectMeta{
			Name: provisioner,
		},
		Spec: crdv1alpha1.ProvisionerCapabilitySpec{
			PluginInfo: crdv1alpha1.ProvisionerCapabilitySpecPluginInfo{
				Name:    provisioner,
				Version: "v0.1.0",
			},
			Features: crdv1alpha1.ProvisionerCapabilitySpecFeatures{
				Topology: true,
				Volume: crdv1alpha1.ProvisionerCapabilitySpecFeaturesVolume{
					Create: true,
					Attach: true,
					List:   false,
					Clone:  true,
					Stats:  true,
					Expand: crdv1alpha1.ExpandModeOffline,
				},
				Snapshot: crdv1alpha1.ProvisionerCapabilitySpecFeaturesSnapshot{
					Create: true,
					List:   false,
				},
			},
		},
	}
}

func newStorageClass(name string, provisioner string) *storagev1.StorageClass {
	isExpansion := true
	return &storagev1.StorageClass{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Provisioner:          provisioner,
		AllowVolumeExpansion: &isExpansion,
	}
}

func newSnapshotClass(name string, provisioner string) *snapbeta1.VolumeSnapshotClass {
	return &snapbeta1.VolumeSnapshotClass{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Driver: provisioner,
	}
}

func (f *fixture) newController() (*Controller, kubeinformers.SharedInformerFactory,
	crdinformers.SharedInformerFactory, snapinformers.SharedInformerFactory) {
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobject...)
	f.crdclient = crdfake.NewSimpleClientset(f.crdobject...)
	f.snapclient = snapfake.NewSimpleClientset(f.snapobject...)

	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	crdI := crdinformers.NewSharedInformerFactory(f.crdclient, noResyncPeriodFunc())
	snapI := snapinformers.NewSharedInformerFactory(f.snapclient, noResyncPeriodFunc())

	c := NewController(f.kubeclient, f.crdclient,
		k8sI.Storage().V1().StorageClasses(),
		snapI.Snapshot().V1beta1().VolumeSnapshotClasses(),
		crdI.Storage().V1alpha1().ProvisionerCapabilities(), crdI.Storage().V1alpha1().StorageClassCapabilities())

	c.sccapSynced = alwaysReady
	c.snapSynced = alwaysReady
	c.pcapSynced = alwaysReady
	c.sccapSynced = alwaysReady

	for _, sc := range f.scLister {
		k8sI.Storage().V1().StorageClasses().Informer().GetIndexer().Add(sc)
	}
	for _, snap := range f.snapLister {
		snapI.Snapshot().V1beta1().VolumeSnapshotClasses().Informer().GetIndexer().Add(snap)
	}
	for _, pcap := range f.pcapLister {
		crdI.Storage().V1alpha1().ProvisionerCapabilities().Informer().GetIndexer().Add(pcap)
	}
	for _, sccap := range f.sccapLister {
		crdI.Storage().V1alpha1().StorageClassCapabilities().Informer().GetIndexer().Add(sccap)
	}
	return c, k8sI, crdI, snapI
}

func (f *fixture) runController(scName string, startInformers bool, expectError bool) {
	c, k8sI, crdI, snapI := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		k8sI.Start(stopCh)
		crdI.Start(stopCh)
		snapI.Start(stopCh)
	}

	err := c.syncHandler(scName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range actions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.kubeactions), actions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(actions), f.kubeactions[len(actions):])
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "storageclasses") ||
				action.Matches("watch", "storageclasses") ||
				action.Matches("list", "provisionercapabilities") ||
				action.Matches("watch", "provisionercapabilities")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

func (f *fixture) run(scName string) {
	f.runController(scName, true, false)
}

func (f *fixture) expectCreateStorageClassCapabilitiesAction(sc *storagev1.StorageClass) {
	f.crdaction = append(f.crdaction, core.NewCreateAction(
		schema.GroupVersionResource{Resource: "storageclasscapabilities"}, sc.Namespace, sc))
}

func TestCreateStorageClass(t *testing.T) {
	f := newFixture(t)
	sc := newStorageClass("sc-example", "csi.example.com")

	f.scLister = append(f.scLister, sc)
	f.kubeobject = append(f.kubeobject, sc)

	f.expectCreateStorageClassCapabilitiesAction(sc)
	f.run(getKey(sc, t))
}

func getKey(sc *storagev1.StorageClass, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(sc)
	if err != nil {
		t.Errorf("Unexpected error getting key for foo %v: %v", sc.Name, err)
		return ""
	}
	return key
}
