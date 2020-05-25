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
