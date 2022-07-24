package framework

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/storage"
	"elasticgpu.io/elastic-gpu/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type GPUPluginConfig struct {
	DeviceLocator map[v1.ResourceName]kube.DeviceLocator
	Sitter        kube.Sitter
	Storage       storage.Storage
	GPUPluginName string
	Client        *kubernetes.Clientset
	EGPUClient    *versioned.Clientset
	NodeName      string
	IsPrivateMode bool
	LSEndpoint    string
}

var (
	RegisteredPlugins map[string]GPUPlugin
)

func RegisterPlugin(plugin GPUPlugin) {
	if RegisteredPlugins == nil {
		RegisteredPlugins = make(map[string]GPUPlugin, 0)
	}
	RegisteredPlugins[plugin.Name()] = plugin
}

type Device struct {
	ID       string
	GPUIndex string
	Paths    []string
	// MB
	Memory uint64
	Health string
}

type DeviceSpec struct {
	ContainerPath string
	HostPath      string
	Permissions   string
}

type Mount struct {
	ContainerPath string
	HostPath      string
}

// ContainerAllocate is a simple wrapper for ContainerAllocateResponse
type ContainerAllocate struct {
	Envs        map[string]string
	Mounts      []*Mount
	Devices     []*DeviceSpec
	Annotations map[string]string
}

type GPUPlugin interface {
	Name() string
	InterestedResources() []v1.ResourceName

	List() []*Device
	Allocate(resource v1.ResourceName, devicesIDs []string) *ContainerAllocate

	Create(index int, id string) error
	Delete(index int, id string) error
}
