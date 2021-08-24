package main

import (
	"flag"
	"github.com/nano-gpu/nano-gpu-agent/pkg/utils"
	"k8s.io/klog"

	"github.com/nano-gpu/nano-gpu-agent/pkg/manager"
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
	go utils.DumpSignal()
	<-utils.ExitSignal()
}
