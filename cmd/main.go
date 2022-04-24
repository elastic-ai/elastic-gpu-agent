package main

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/manager"
	"flag"
	"k8s.io/klog"
)

var (
	node          string
	dbfile        string
	gpuPluginName string
	kubeconf      string
)

func main() {
	flag.StringVar(&node, "nodeName", "", "current nodename")
	flag.StringVar(&dbfile, "dbFile", "", "database path")
	flag.StringVar(&kubeconf, "kubeconf", "", "kubeconfig path")
	flag.StringVar(&gpuPluginName, "gpuPluginName", "qgpu", "gpu plugin name, possible values: 'gpushare'")
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	gpumanager, err := manager.NewGPUManager(manager.WithNodeName(node), manager.WithDBPath(dbfile), manager.WithKubeconf(kubeconf), manager.WithGPUPluginName(gpuPluginName))
	if err != nil {
		klog.Fatalln(err.Error())
		return
	}
	klog.Info("start to run elastic gpu agent")
	gpumanager.Run()
	go common.DumpSignal()
	<-common.ExitSignal()
}
