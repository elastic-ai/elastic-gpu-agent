package gpu

import (
	"github.com/nano-gpu/nano-gpu-agent/pkg/types"
	"k8s.io/api/core/v1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func NewGPUShare() *GPUShare {
	return &GPUShare{}
}

// TODO
type GPUShare struct {
}

func (g *GPUShare) GetGPUCount() (int, error) {
	return 0, nil
}

func (g *GPUShare) GetGPUMemory(index int) (int, error) {
	return 0, nil
}

func (g *GPUShare) Allocate(devices types.DeviceList) ([]*pluginapi.ContainerAllocateResponse, error) {
	return nil, nil
}

func (g *GPUShare) CreateGPUGrid(gpuID int, resources v1.ResourceList, devices types.DeviceList) error {
	return nil
}

func (g *GPUShare) DeleteGPUGrid(gpuID int, devices types.DeviceList) error {
	return nil
}
