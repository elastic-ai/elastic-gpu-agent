package plugins

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"fmt"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"strconv"
	"strings"
)

type GPUShareDevicePlugin struct {
	baseDevicePlugin
}

func NewGPUShareDevicePlugin(config *GPUPluginConfig) (pluginapi.DevicePluginServer, error) {
	devs, err := config.GPUOperator.Devices()
	if err != nil {
		return nil, err
	}
	devices := make([]*pluginapi.Device, 0)
	for i, d := range devs {
		for j := uint64(0); j < d.Memory/1024/1024; j++ {
			devices = append(devices, &pluginapi.Device{
				ID:     fmt.Sprintf("%d-%02d", i, j),
				Health: pluginapi.Healthy,
			})
		}
	}
	return &GPUShareDevicePlugin{
		baseDevicePlugin: baseDevicePlugin{devices: devices, GPUPluginConfig: config},
	}, nil
}

func (c *GPUShareDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (c *GPUShareDevicePlugin) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: c.devices}); err != nil {
		klog.Error(err)
		return err
	}
	<-server.Context().Done()
	return nil
}

func (c *GPUShareDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devicesIDs := []string{}
	for _, container := range request.ContainerRequests {
		devicesIDs = append(devicesIDs, container.DevicesIDs...)
	}
	if len(devicesIDs) == 0 {
		return &pluginapi.AllocateResponse{}, fmt.Errorf("devices is empty")
	}
	device := types.NewDevice(devicesIDs)
	faker := device.Hash
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs: map[string]string{
				"GPU":                    faker,
				"NVIDIA_VISIBLE_DEVICES": "none",
			},
			Mounts: []*pluginapi.Mount{},
			Devices: []*pluginapi.DeviceSpec{
				{
					ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s", faker),
					HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s", faker),
					Permissions:   "rwm",
				},
				{
					ContainerPath: "/host/dev/nvidiactl",
					HostPath:      "/dev/nvidiactl",
					Permissions:   "rwm",
				}, {
					ContainerPath: "/host/dev/nvidia-uvm",
					HostPath:      "/dev/nvidia-uvm",
					Permissions:   "rwm",
				}, {
					ContainerPath: "/host/dev/nvidia-uvm-tools",
					HostPath:      "/dev/nvidia-uvm-tools",
					Permissions:   "rwm",
				},
			},
			Annotations: map[string]string{},
		}},
	}, nil
}

func (c *GPUShareDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if len(request.DevicesIDs) == 0 {
		return &pluginapi.PreStartContainerResponse{}, fmt.Errorf("find empty device list")
	}
	devices := types.NewDevice(request.DevicesIDs)
	curr, err := c.DeviceLocator.Locate(devices)
	if err != nil {
		klog.Errorf("no pod with such device list: %s", strings.Join(request.DevicesIDs, ":"))
		return nil, err
	}
	pod, err := c.Sitter.GetPod(curr.Namespace, curr.Name)
	if err != nil {
		klog.Errorf("failed to get pod %s: %s", curr, err.Error())
		return nil, err
	}
	if _, ok := pod.Annotations[common.ElasticGPUAssumedAnnotation]; !ok {
		klog.Errorf("annotation %s does not on pod %s", common.ElasticGPUAssumedAnnotation, curr)
		return nil, fmt.Errorf("annotation %s does not on pod %s", common.ElasticGPUAssumedAnnotation, curr)
	}
	containerAssumedKey := fmt.Sprintf(common.ElasticGPUContainerAnnotation, curr.Container)
	val, ok := pod.Annotations[containerAssumedKey]
	if !ok {
		klog.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, curr, curr.Container)
		return nil, fmt.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, curr, curr.Container)
	}
	idx, err := strconv.Atoi(val)
	if err != nil {
		klog.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
		return nil, fmt.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.GPUOperator.Create(idx, devices.Hash); err != nil {
		klog.Errorf(err.Error())
		return nil, err
	}
	err = nil
	defer func() {
		if err != nil {
			klog.Error(err.Error())
			if deleteError := c.GPUOperator.Delete(idx, devices.Hash); deleteError != nil {
				klog.Errorf("%s delete elastic gpu failed: %s, delete reason: %s", curr.String(), deleteError.Error(), err.Error())
			}
		}
	}()
	pif := c.Storage.LoadOrCreate(curr.Namespace, curr.Name)
	pif.ContainerDeviceMap[curr.Container] = devices
	err = c.Storage.Save(pif)
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (c *GPUShareDevicePlugin) GetPreferredAllocation(ctx context.Context, request *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

type GPUShareCoreDevicePlugin struct {
	baseDevicePlugin
}

func NewGPUShareCoreDevicePlugin(config *GPUPluginConfig) (*GPUShareCoreDevicePlugin, error) {
	devs, err := config.GPUOperator.Devices()
	if err != nil {
		return nil, err
	}
	devices := make([]*pluginapi.Device, 0)
	for i, _ := range devs {
		for j := uint64(0); j < common.GPUPercentEachCard; j++ {
			devices = append(devices, &pluginapi.Device{
				ID:     fmt.Sprintf("%d-%02d", i, j),
				Health: pluginapi.Healthy,
			})
		}
	}
	return &GPUShareCoreDevicePlugin{
		baseDevicePlugin: baseDevicePlugin{devices: devices, GPUPluginConfig: config},
	}, nil
}
