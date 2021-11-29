package plugins

import (
	"context"
	"fmt"
	"manager/pkg/common"
	"strconv"
	"strings"
	"sync"

	"manager/pkg/kube"

	"manager/pkg/storage"

	"manager/pkg/nvidia"
	"manager/pkg/types"

	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type NanoServer struct {
	devices  []*pluginapi.Device
	locator  kube.DeviceLocator
	sitter   kube.Sitter
	operator nvidia.GPUOperator
	storage  storage.Storage
	lock     sync.Mutex
}

func NewNanoServer(locator kube.DeviceLocator, sitter kube.Sitter, operator nvidia.GPUOperator, storage storage.Storage) (pluginapi.DevicePluginServer, error) {
	count, err := operator.DetectNumber()
	if err != nil {
		return nil, err
	}
	devices := []*pluginapi.Device{}
	for i := 0; i < count; i++ {
		for j := 0; j < common.GPUPercentEachCard; j++ {
			devices = append(devices, &pluginapi.Device{
				ID:     fmt.Sprintf("nano-%d-%02d", i, j),
				Health: pluginapi.Healthy,
			})
		}
	}
	return &NanoServer{
		devices:  devices,
		operator: operator,
		storage:  storage,
		sitter:   sitter,
		locator:  locator,
	}, nil
}

func (c *NanoServer) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (c *NanoServer) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: c.devices}); err != nil {
		klog.Error(err)
		return err
	}
	<-server.Context().Done()
	return nil
}

func (c *NanoServer) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var devices *types.Device
	for _, container := range request.ContainerRequests {
		if len(container.DevicesIDs) > 0 {
			devices = types.NewDevice(container.DevicesIDs)
			klog.Infof("AllocateRequest sorted DevicesIDs: %v", devices.List)
		}
	}
	if devices == nil {
		return nil, fmt.Errorf("empty device list")
	}
	faker := devices.Hash
	return &pluginapi.AllocateResponse{
		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
			Envs: map[string]string{
				"GPU":                    faker,
				"NVIDIA_VISIBLE_DEVICES": "none",
				"ku":                     fmt.Sprintf("%d", len(devices.List)),
			},
			Mounts: []*pluginapi.Mount{},
			Devices: []*pluginapi.DeviceSpec{
				{
					ContainerPath: fmt.Sprintf("/host/dev/nano-gpu-%s", faker),
					HostPath:      fmt.Sprintf("/dev/nano-gpu-%s", faker),
					Permissions:   "rwm",
				}, {

					ContainerPath: fmt.Sprintf("/host/dev/nano-gpuctl-%s", faker),
					HostPath:      fmt.Sprintf("/dev/nano-gpuctl-%s", faker),
					Permissions:   "rwm",
				},
			},
			Annotations: map[string]string{},
		}},
	}, nil
}

func (c *NanoServer) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if len(request.DevicesIDs) == 0 {
		klog.Errorln("empty device list")
		return nil, fmt.Errorf("empty device list")
	}

	devices := types.NewDevice(request.DevicesIDs)
	klog.Infof("PreStartContainerRequest sorted DeviceIDs: %v", devices.List)

	// 1. locate the pod
	curr, err := c.locator.Locate(devices)
	if err != nil {
		klog.Errorf("no pod with such device list: %s", strings.Join(request.DevicesIDs, ":"))
		return nil, err
	}
	pod, err := c.sitter.GetPod(curr.Namespace, curr.Name)
	if err != nil {
		klog.Errorf("get pod %s failed: %s", curr, err.Error())
		return nil, err
	}
	klog.Infof("get pod %s/%s", pod.Namespace, pod.Name)
	// 2. check pod annotation
	if _, ok := pod.Annotations[common.NanoGPUAssumedAnnotation]; !ok {
		klog.Errorf("annotation %s does not on pod %s", common.NanoGPUAssumedAnnotation, curr)
		return nil, fmt.Errorf("annotation %s does not on pod %s", common.NanoGPUAssumedAnnotation, curr)
	}
	containerAssumedKey := fmt.Sprintf(common.NanoGPUContainerAnnotation, curr.Container)
	val, ok := pod.Annotations[containerAssumedKey]
	if !ok {
		klog.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, curr, curr.Container)
		return nil, fmt.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, curr, curr.Container)
	}

	// 3. get pod gpu id
	idx, err := strconv.Atoi(val)
	if err != nil {
		klog.Errorf("the %s assumed on pod %s, container %s may be not nano gpu index", val, curr.Pod(), curr.Container)
		return nil, fmt.Errorf("the %s assumed on pod %s, container %s may be not nano gpu index", val, curr.Pod(), curr.Container)
	}

	// 4. create gpu and record it
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.operator.Create(idx, devices.Hash); err != nil {
		klog.Errorf(err.Error())
		return nil, err
	}
	err = nil
	defer func() {
		if err != nil {
			klog.Error(err.Error())
			if deleteError := c.operator.Delete(idx, devices.Hash); deleteError != nil {
				klog.Errorf("%s delete nano gpu failed: %s, delete reason: %s", curr.String(), deleteError.Error(), err.Error())
			}
		}
	}()
	pif := c.storage.LoadOrCreate(curr.Namespace, curr.Name)
	pif.ContainerDeviceMap[curr.Container] = devices
	err = c.storage.Save(pif)
	return &pluginapi.PreStartContainerResponse{}, err
}

func (c *NanoServer) GetPreferredAllocation(ctx context.Context, request *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}
