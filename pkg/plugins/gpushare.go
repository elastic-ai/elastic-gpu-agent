package plugins

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"elasticgpu.io/elastic-gpu/api/v1alpha1"
	"fmt"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"strconv"
	"strings"
)

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
	return &GPUShareCoreDevicePlugin{baseDevicePlugin: baseDevicePlugin{ResourceName: v1alpha1.ResourceGPUCore, devices: devices, GPUPluginConfig: config}}, nil
}

func (c *GPUShareCoreDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devicesIDs := []string{}
	for _, container := range request.ContainerRequests {
		devicesIDs = append(devicesIDs, container.DevicesIDs...)
	}
	if len(devicesIDs) == 0 {
		return &pluginapi.AllocateResponse{}, fmt.Errorf("devices is empty")
	}
	device := types.NewDevice(devicesIDs)
	faker := device.Hash

	devices := []*pluginapi.DeviceSpec{
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
	}
	if len(devicesIDs) > 100 {
		for i := 0; i < len(devicesIDs)/100; i++ {
			devices = append(devices, &pluginapi.DeviceSpec{
				ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s-%d", faker, i),
				HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s-%d", faker, i),
				Permissions:   "rwm",
			})
		}
	} else {
		devices = append(devices, &pluginapi.DeviceSpec{
			ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s-%d", faker, 0),
			HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s-%d", faker, 0),
			Permissions:   "rwm",
		})
	}
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs: map[string]string{
				"GPU":                    faker,
				"NVIDIA_VISIBLE_DEVICES": "none",
			},
			Devices: devices,
		}},
	}, nil
}

func (c *GPUShareCoreDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if len(request.DevicesIDs) == 0 {
		return &pluginapi.PreStartContainerResponse{}, fmt.Errorf("find empty device list")
	}
	devices := types.NewDevice(request.DevicesIDs)
	curr, err := c.DeviceLocator[c.ResourceName].Locate(devices)
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

	c.lock.Lock()
	defer c.lock.Unlock()
	ids := strings.Split(val, ",")
	idInts := make([]int, 0)
	for _, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			klog.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
			return nil, fmt.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
		}
		idInts = append(idInts, id)
	}
	klog.V(5).Infof("allocated gpu %+v on container %s/%s/%s", ids, curr.Namespace, curr.Name, curr.Container)
	for i, id := range idInts {
		if err := c.GPUOperator.Create(id, fmt.Sprintf("%s-%d", devices.Hash, i)); err != nil {
			klog.Errorf(err.Error())
			return nil, err
		}
	}
	defer func() {
		if err != nil {
			klog.Error(err.Error())
			for i, id := range idInts {
				if deleteError := c.GPUOperator.Delete(id, fmt.Sprintf("%s-%d", devices.Hash, i)); deleteError != nil {
					klog.Errorf("%s delete elastic gpu failed: %s, delete reason: %s", curr.String(), err.Error(), deleteError.Error())
				}
			}
		}
	}()

	pif := c.Storage.LoadOrCreate(curr.Namespace, curr.Name)
	pif.ContainerDeviceMap[curr.Container] = devices
	err = c.Storage.Save(pif)
	return &pluginapi.PreStartContainerResponse{}, nil
}

type GPUShareMemoryDevicePlugin struct {
	baseDevicePlugin
}

func NewGPUShareMemoryDevicePlugin(config *GPUPluginConfig) (pluginapi.DevicePluginServer, error) {
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
	return &GPUShareMemoryDevicePlugin{baseDevicePlugin: baseDevicePlugin{ResourceName: v1alpha1.ResourceGPUMemory, devices: devices, GPUPluginConfig: config}}, nil
}

func (c *GPUShareMemoryDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devicesIDs := []string{}
	for _, container := range request.ContainerRequests {
		devicesIDs = append(devicesIDs, container.DevicesIDs...)
	}
	if len(devicesIDs) == 0 {
		return &pluginapi.AllocateResponse{}, fmt.Errorf("devices is empty")
	}
	device := types.NewDevice(devicesIDs)
	faker := device.Hash

	devices := []*pluginapi.DeviceSpec{
		{
			ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s-0", faker),
			HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s-0", faker),
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
	}
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs: map[string]string{
				"GPU":                    faker,
				"NVIDIA_VISIBLE_DEVICES": "none",
			},
			Devices: devices,
		}},
	}, nil
}

func (c *GPUShareMemoryDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if len(request.DevicesIDs) == 0 {
		return &pluginapi.PreStartContainerResponse{}, fmt.Errorf("find empty device list")
	}
	devices := types.NewDevice(request.DevicesIDs)
	curr, err := c.DeviceLocator[c.ResourceName].Locate(devices)
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

	c.lock.Lock()
	defer c.lock.Unlock()
	id, err := strconv.Atoi(val)
	if err != nil {
		klog.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
		return nil, fmt.Errorf("the %s assumed on pod %s, container %s may be not elastic gpu index", val, curr.Pod(), curr.Container)
	}
	klog.V(5).Infof("allocated gpu %+v on container %s/%s/%s", id, curr.Namespace, curr.Name, curr.Container)
	if err := c.GPUOperator.Create(id, fmt.Sprintf("%s-%d", devices.Hash, 0)); err != nil {
		klog.Errorf(err.Error())
		return nil, err
	}
	defer func() {
		if err != nil {
			klog.Error(err.Error())
			if deleteError := c.GPUOperator.Delete(id, fmt.Sprintf("%s-%d", devices.Hash, 0)); deleteError != nil {
				klog.Errorf("%s delete elastic gpu failed: %s, delete reason: %s", curr.String(), err.Error(), deleteError.Error())
			}
		}
	}()

	pif := c.Storage.LoadOrCreate(curr.Namespace, curr.Name)
	pif.ContainerDeviceMap[curr.Container] = devices
	err = c.Storage.Save(pif)
	return &pluginapi.PreStartContainerResponse{}, nil
}
