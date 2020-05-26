# Storage Capability

## Overview

This repository implements a controller and sidecar for gathering storage class and storage plugin capabilities through CustomResourceDefinition (CRD). Because Kubernetes snapshot goes into Beta in Kubernetes v1.17, this controller only support Kubernetes v1.17+.

## Build

This command will build controller and sidecar binary and container image simultaneously.
```
make
```

## Installation

### Prerequsite

- Kubernetes v1.17+
- Install [Kubernetes Snapshot Beta CRDs](https://github.com/kubernetes-csi/external-snapshotter#usage)

### Install CRDs

```
kubectl create -f crd/storage-v1alpha1-class-cap.yaml
kubectl create -f crd/storage-v1alpha1-provisioner-cap.yaml
```

### Install Controller

The controller will watch StorageClass, VolumeSnapshotClass, ProvisionerCapability CRD and StorageClassCapability CRD and update StorageClassCapability CRD.
```
kubectl create -f deploy/controller-rbac.yaml
kubectl create -f deploy/controller-deploy.yaml
``` 

### Install Webhook

The webhook will add sidecar container and ClusterRoleBinding when deploying CSI plugin with special annotation.
```
./deploy/webhook/deploy.sh
```

## Usage
People should add three annotations to CSI controller Pods to enable storage capability and specify storage capability parameters. We also provide an [example](./example) to config CSI plugin.
- storage.kubesphere.io/storage-capability-address: the address of CSI socket. same as external provisioner or external attacher container's CSI address.
- storage.kubesphere.io/storage-capability-mount-path: the path to mount. same as the CSI socket volume mount path in external provisioner or external attacher container.
- storage.kubesphere.io/storage-capability-volume-name: the volume to mount. same as the CSI socket volume name in external provisioner or external attacher container.

```
apiVersion: v1
kind: Pod
metadata:
  annotations:
    ...
    storage.kubesphere.io/storage-capability-address: /csi/csi.sock
    storage.kubesphere.io/storage-capability-mount-path: /csi
    storage.kubesphere.io/storage-capability-volume-name: socket-dir
  name: csi-example-controller-c58c54f45-j5mq9
  namespace: example
spec:
  ...
```

## Uninstallation

```
./deploy/webhook/undeploy.sh
kubectl delete -f deploy/controller-deploy.yaml
kubectl delete -f deploy/controller-rbac.yaml
```