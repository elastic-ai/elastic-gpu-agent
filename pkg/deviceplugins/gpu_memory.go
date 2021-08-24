package deviceplugins

import (
	"context"
	"fmt"
	"github.com/nano-gpu/nano-gpu-agent/pkg/gpu"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type NanoGPUMemoryDevicePlugin struct {
	baseDevicePlugin
}

func NewNanoGPUMemoryDevicePlugin(operator gpu.GPUOperator) (pluginapi.DevicePluginServer, error) {
	count, err := operator.GetGPUCount()
	if err != nil {
		return nil, err
	}
	devices := []*pluginapi.Device{}
	for i := 0; i < count; i++ {
		memory, err := operator.GetGPUMemory(i)
		if err != nil {
			return nil, err
		}
		for j := 0; j < memory; j++ {
			devices = append(devices, &pluginapi.Device{
				ID:     fmt.Sprintf("%d-%02d", i, j),
				Health: pluginapi.Healthy,
			})
		}
	}
	return &NanoGPUMemoryDevicePlugin{
		baseDevicePlugin{
			PreStartRequired: false,
			DeviceList:       devices,
		},
	}, nil
}

func (c *NanoGPUMemoryDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	size := 0
	for _, container := range request.ContainerRequests {
		size += len(container.DevicesIDs)
	}
	// payall4u: QWEIGTH is GB or MB or others?
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs:        map[string]string{"QMEMSIZE": fmt.Sprintf("%d", size)},
			Mounts:      nil,
			Devices:     nil,
			Annotations: nil,
		}},
	}, nil
}

func (c *NanoGPUMemoryDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}
