package framework

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"strconv"
	"strings"
	"sync"
)

type baseDevicePlugin struct {
	ResourceName v1.ResourceName
	plugin       GPUPlugin
	*GPUPluginConfig

	PreStartRequired bool
	lock             sync.Mutex
}

func NewDevicePlugin(c *GPUPluginConfig, resourceName v1.ResourceName, plugin GPUPlugin) v1beta1.DevicePluginServer {
	return &baseDevicePlugin{GPUPluginConfig: c, ResourceName: resourceName, plugin: plugin}
}

func (m *baseDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (m *baseDevicePlugin) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	devices := m.plugin.List()
	apiDevices := make([]*v1beta1.Device, 0)
	for _, d := range devices {
		apiDevices = append(apiDevices, &v1beta1.Device{
			ID:     d.ID,
			Health: d.Health,
		})
	}
	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: apiDevices}); err != nil {
		return err
	}
	// Sync GPUs on node.
	syncer := NewGPUSyncer(m.GPUPluginConfig)
	if err := syncer.Sync(); err != nil {
		klog.Errorf("Fail to sync gpus on node %s: %v", m.NodeName, err)
	}
	<-server.Context().Done()

	return nil
}

func (m *baseDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devicesIDs := []string{}
	for _, container := range request.ContainerRequests {
		devicesIDs = append(devicesIDs, container.DevicesIDs...)
	}
	if len(devicesIDs) == 0 {
		return &pluginapi.AllocateResponse{}, fmt.Errorf("devices is empty")
	}

	allocate := m.plugin.Allocate(m.ResourceName, devicesIDs)
	devices := make([]*v1beta1.DeviceSpec, 0)
	for _, d := range allocate.Devices {
		devices = append(devices, &v1beta1.DeviceSpec{
			ContainerPath: d.ContainerPath,
			HostPath:      d.HostPath,
		})
	}
	mounts := make([]*v1beta1.Mount, 0)
	for _, m := range allocate.Mounts {
		mounts = append(mounts, &v1beta1.Mount{
			ContainerPath: m.ContainerPath,
			HostPath:      m.HostPath,
		})
	}
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs:        allocate.Envs,
			Devices:     devices,
			Mounts:      mounts,
			Annotations: allocate.Annotations,
		}},
	}, nil
}

func (m *baseDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if len(request.DevicesIDs) == 0 {
		return &pluginapi.PreStartContainerResponse{}, fmt.Errorf("find empty device list")
	}
	devices := types.NewDevice(request.DevicesIDs, m.ResourceName)
	curr, err := m.DeviceLocator[m.ResourceName].Locate(devices)
	if err != nil {
		klog.Errorf("no pod with such device list: %s", strings.Join(request.DevicesIDs, ":"))
		return nil, err
	}
	pod, err := m.Sitter.GetPod(curr.Namespace, curr.Name)
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

	m.lock.Lock()
	defer m.lock.Unlock()
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
		if err := m.plugin.Create(id, fmt.Sprintf("%s-%d", devices.Hash, i)); err != nil {
			klog.Errorf(err.Error())
			return nil, err
		}
	}
	defer func() {
		if err != nil {
			klog.Error(err.Error())
			for i, id := range idInts {
				if deleteError := m.plugin.Delete(id, fmt.Sprintf("%s-%d", devices.Hash, i)); deleteError != nil {
					klog.Errorf("%s delete elastic gpu failed: %s, delete reason: %s", curr.String(), err.Error(), deleteError.Error())
				}
			}
		}
	}()

	pif := m.Storage.LoadOrCreate(curr.Namespace, curr.Name)
	pif.ContainerDeviceMap[curr.Container] = devices
	err = m.Storage.Save(pif)
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *baseDevicePlugin) GetPreferredAllocation(ctx context.Context, request *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}
