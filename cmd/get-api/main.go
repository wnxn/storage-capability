package main

import (
	"flag"
	clientset "github.com/kubesphere/storage-capability/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	scList, err := kubeClient.StorageV1().StorageClasses().List(metav1.ListOptions{})
	if err != nil {
		klog.Fatal("sc client set error: ", err.Error())
	}
	for _, item := range scList.Items {
		klog.Info(item.GetName())
	}

	pcapList, err := exampleClient.StorageV1alpha1().ProvisionerCapabilities().List(metav1.ListOptions{})
	if err != nil {
		klog.Fatal("sccap client set error: ", err.Error())
	}
	klog.Info(len(pcapList.Items))

	sccapList, err := exampleClient.StorageV1alpha1().StorageClassCapabilities().List(metav1.ListOptions{})
	if err != nil {
		klog.Fatal("sccap client set error: ", err.Error())
	}
	klog.Info(len(sccapList.Items))
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
