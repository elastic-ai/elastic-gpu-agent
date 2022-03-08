package main

import (
	"flag"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"

	"k8s.io/klog"

	"elasticgpu.io/elastic-gpu-agent/pkg/manager"
)

var (
	node   string
	dbfile string
)

func init() {
	flag.StringVar(&node, "nodename", "", "current nodename")
	flag.StringVar(&dbfile, "dbfile", "", "database path")
	flag.Parse()
}

func main() {
	klog.InitFlags(flag.CommandLine)
	gpumanager, err := manager.NewGPUManager(manager.WithNodeName(node), manager.WithDBPath(dbfile))
	if err != nil {
		klog.Fatalln(err.Error())
		return
	}
	gpumanager.Run()
	go common.DumpSignal()
	<-common.ExitSignal()
}
