package deviceplugins

import (
	"fmt"
	"github.com/nano-gpu/nano-gpu-agent/pkg/gpu"
	"github.com/nano-gpu/nano-gpu-agent/pkg/kube"
	"github.com/nano-gpu/nano-gpu-agent/pkg/storage"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type NanoGPUCoreDevicePlugin struct {
	baseDevicePlugin
}

func NewNanoGPUCoreDevicePlugin(locator kube.DeviceLocator, sitter kube.Sitter, operator gpu.GPUOperator, storage storage.Storage) (pluginapi.DevicePluginServer, error) {
	count, err := operator.GetGPUCount()
	if err != nil {
		return nil, err
	}
	devices := []*pluginapi.Device{}
	for i := 0; i < count; i++ {
		for j := 0; j < types.GPUCoreEachCard; j++ {
			devices = append(devices, &pluginapi.Device{
				ID:     fmt.Sprintf("pgpu%d-%02d", i, j),
				Health: pluginapi.Healthy,
			})
		}
	}
	return &NanoGPUCoreDevicePlugin{
		baseDevicePlugin: baseDevicePlugin{
			PreStartRequired: true,
			DeviceList:       devices,
			operator:         operator,
			storage:          storage,
			sitter:           sitter,
			locator:          locator,
		},
	}, nil
}
