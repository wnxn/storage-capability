package main

import (
	"flag"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	clientset "github.com/wnxn/storage-capability/pkg/generated/clientset/versioned"
	"github.com/wnxn/storage-capability/pkg/sidecar"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"os"
	"os/signal"
	"time"
)

const (
	// Default timeout of short CSI calls like GetPluginInfo
	defaultTimeout = time.Minute
	version        = "unknown"
)

var (
	masterURL      string
	kubeconfig     string
	csiAddress     = flag.String("csi-address", "/run/csi/socket", "Address of the CSI driver socket.")
	csiNodeAddress = flag.String("csi-node-address", "", "Address of the CSI Node driver socket.")
	timeout        = flag.Duration("timeout", defaultTimeout, "The timeout for any RPCs to the CSI driver. Default is 1 minute.")
	resyncPeriod   = flag.Duration("resync-period", 60*time.Second, "Resync interval of the controller.")
	driverName     = flag.String("driver-name", "", "The provisioner name of plugin")
)

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	klog.Infof("Version: %s", version)

	// set csi node address
	if *csiNodeAddress == "" {
		*csiNodeAddress = *csiAddress
	}
	// Create Kubernetes CRD client
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	clientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
	// Create CSI gRPC client connection
	metricsManager := metrics.NewCSIMetricsManager("" /* driverName */)
	csiConn, err := connection.Connect(*csiAddress, metricsManager, connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
	if err != nil {
		klog.Errorf("error connecting to CSI driver: %v", err)
		os.Exit(1)
	}
	controller := sidecar.NewCSISidecarController(
		clientset,
		csiConn,
		*driverName,
		*timeout,
		*resyncPeriod,
	)
	stopCh := make(chan struct{})
	go controller.Run(stopCh)
	// ...until SIGINT
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	close(stopCh)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
