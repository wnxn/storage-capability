# Storage Capability

## Overview

This repository implements a controller and sidecar for gathering storage class and storage plugin capabilities through CustomResourceDefinition (CRD). Because Kubernetes snapshot goes into Beta in Kubernetes v1.17, this controller only support Kubernetes v1.17+.

## Build

This command will build controller and sidecar binary and container image simultaneously.
```
make
```

## Usage

### Prerequsite

- Kubernetes v1.17+
- Install [Kubernetes Snapshot Beta CRDs](https://github.com/kubernetes-csi/external-snapshotter#usage)

### Install CRDs

```
kubectl create -f crd/storage-v1alpha1-class-cap.yaml
kubectl create -f crd/storage-v1alpha1-provisioner-cap.yaml
```

### Controller

Install one controller in a Kubernetes v1.17+ cluster.
```
kubectl create -f deploy/controller-rbac.yaml
kubectl create -f deploy/controller-deploy.yaml
``` 

### CSI Plugin Sidecar

Add sidecar rbac and container in CSI plugin controller server Pod.
- RBAC
```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: storage-capability
    owner: yunify
    role: sidecar
    ver: v0.1.0
  name: storage-capability-sidecar
rules:
  - apiGroups:
      - "storage.kubesphere.io"
    resources:
      - provisionercapabilities
    verbs:
      - create
      - get
      - list
      - watch
      - update
      - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app: storage-capability
    owner: yunify
    role: sidecar
    ver: v0.1.0
  name: storage-capability-sidecar
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: storage-capability-sidecar
subjects:
  - kind: ServiceAccount
    name: <CSI PLUGIN SERVICE ACCOUNT NAME>
    namespace: kube-system
```

- Container
```
      containers:
        - args:
            - --csi-address=$(ADDRESS)
            - --driver-name="<drivername.example.com>"
            - --v=5
          env:
            - name: ADDRESS
              value: /csi/csi.sock
          image: wangxinsh/storage-capability-sidecar:v0.1.0
          name: sidecar
          resources:
            limits:
              cpu: 80m
              memory: 80Mi
            requests:
              cpu: 80m
              memory: 80Mi
          volumeMounts:
            - mountPath: /csi
              name: socket-dir
      serviceAccount: <CSI PLUGIN SERVICE ACCOUNT NAME>
      volumes:
        - emptyDir: null
          name: socket-dir
```

## Uninstallation

```
kubectl delete -f deploy/controller-deploy.yaml
kubectl delete -f deploy/controller-rbac.yaml
```