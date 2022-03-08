package common

const (
	GPUPercentEachCard         = 100
	NanoGPUContainerAnnotation = "elastic-gpu/container-%s"
	NanoGPUAssumedAnnotation   = "elastic-gpu/assume"
	NanoGPUAssumedLabel        = "elastic-gpu/assume"
	ResourceName               = "elastic-gpu/gpu-percent"
	NanoGPUSock                = "elastic-gpu.sock"
	NodeNameField              = "spec.nodeName"
)
