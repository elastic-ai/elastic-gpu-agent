package config

const (
	GPUCorePluginSock   = "nano-gpu-core.sock"
	GPUMemoryPluginSock = "nano-gpu-memory.sock"
	NodeNameField       = "spec.nodeName"
)

var (
	NeverStop chan struct{}
)
