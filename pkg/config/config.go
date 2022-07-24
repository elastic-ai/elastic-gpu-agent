package config

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/operator"
	"elasticgpu.io/elastic-gpu-agent/pkg/storage"
	"elasticgpu.io/elastic-gpu/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type DevicePluginConfig struct {
	DeviceLocator    kube.DeviceLocator
	Sitter           kube.Sitter
	Storage          storage.Storage
	DevicePluginName string
	Client           *kubernetes.Clientset
	EGPUClient       *versioned.Clientset
	GPUOperator      operator.GPUOperator
	NodeName         string
}
