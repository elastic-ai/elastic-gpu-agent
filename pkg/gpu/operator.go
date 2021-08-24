package gpu

import (
	"github.com/nano-gpu/nano-gpu-agent/pkg/types"
	"k8s.io/api/core/v1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type GPUOperator interface {
	GetGPUCount() (int, error)
	GetGPUMemory(index int) (int, error)
	Allocate(devices types.DeviceList) ([]*pluginapi.ContainerAllocateResponse, error)
	CreateGPUGrid(gpuID int, resources v1.ResourceList, devices types.DeviceList) error
	DeleteGPUGrid(gpuID int, devices types.DeviceList) error
}
