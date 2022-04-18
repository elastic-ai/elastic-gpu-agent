package main

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"flag"
	"k8s.io/client-go/util/homedir"
	"path/filepath"

	"k8s.io/klog"

	"elasticgpu.io/elastic-gpu-agent/pkg/manager"
)

var (
	node       string
	dbfile     string
	kubeconfig string
)

func init() {
	flag.StringVar(&node, "nodename", "", "current nodename")
	flag.StringVar(&dbfile, "dbfile", "", "database path")
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
}

func main() {
	klog.InitFlags(flag.CommandLine)
	client, _ := common.NewClientOutOfCluster(kubeconfig)
	gpumanager, err := manager.NewGPUManager(manager.WithNodeName(node), manager.WithDBPath(dbfile), manager.WithClientset(client))
	if err != nil {
		klog.Fatalln(err.Error())
		return
	}
	gpumanager.Run()
	go common.DumpSignal()
	<-common.ExitSignal()
}
