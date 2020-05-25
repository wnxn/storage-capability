# !/bin/bash
kubectl delete ns webhook-demo
kubectl delete MutatingWebhookConfiguration demo-webhook
kubectl delete ClusterRole storage-capability-webhook
kubectl delete ClusterRoleBinding storage-capability-webhook