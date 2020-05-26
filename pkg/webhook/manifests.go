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

const (
	clusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    owner: storage-capability
    ver: v0.1.0
  name: {{ .UniqueName }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: storage-capability-sidecar
subjects:
  - kind: ServiceAccount
    name: {{ .ServiceAccountName }}
    namespace: {{ .ServiceAccountNamespace }}
`
	clusterRole = `
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
`
	clusterRoleName = "storage-capability-sidecar"
)
