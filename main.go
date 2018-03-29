package main

import (
	"flag"
	//"time"

	"github.com/golang/glog"
	//kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	//clientset "k8s.io/sample-controller/pkg/client/clientset/versioned"
	//informers "k8s.io/sample-controller/pkg/client/informers/externalversions"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL   string
	kubeconfig  string
	clusterName string
)

func main() {
	flag.Parse()

	if clusterName == "" {
		glog.Fatal("clustername is required")
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	//kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	//exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	controller := NewController(kubeClient, clusterName)

	//go kubeInformerFactory.Start(stopCh)
	//go exampleInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&clusterName, "clustername", "", "The name of the Kubernetes cluster server.")
}
