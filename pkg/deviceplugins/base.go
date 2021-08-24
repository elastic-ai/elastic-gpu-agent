package deviceplugins

import (
	"context"
	"fmt"
	"github.com/nano-gpu/nano-gpu-agent/pkg/gpu"
	"github.com/nano-gpu/nano-gpu-agent/pkg/kube"
	"github.com/nano-gpu/nano-gpu-agent/pkg/storage"
	"github.com/nano-gpu/nano-gpu-agent/pkg/types"
	schetypes "github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"k8s.io/api/core/v1"
	"k8s.io/klog"
	"strconv"
	"strings"
	"sync"

	"github.com/nano-gpu/nano-gpu-agent/pkg/config"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type baseDevicePlugin struct {
	locator          kube.DeviceLocator
	sitter           kube.Sitter
	storage          storage.Storage
	lock             sync.Mutex
	operator         gpu.GPUOperator
	PreStartRequired bool
	DeviceList       []*pluginapi.Device
}

func (c *baseDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (c *baseDevicePlugin) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: c.DeviceList}); err != nil {
		return err
	}
	<-config.NeverStop
	return nil
}

func (m *baseDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devices := types.DeviceList{}
	for _, container := range request.ContainerRequests {
		if len(container.DevicesIDs) > 0 {
			devices = container.DevicesIDs
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("empty device list")
	}
	if resp, err := m.operator.Allocate(devices); err != nil {
		return nil, fmt.Errorf("failed to allocate devices")
	} else {
		return &pluginapi.AllocateResponse{ContainerResponses: resp}, nil
	}
}

func (m *baseDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	devices := request.DevicesIDs
	// 1. locate the pod
	info, err := m.locator.Locate(devices)
	if err != nil {
		klog.Errorf("no pod with such device list: %s", strings.Join(devices, ":"))
		return nil, err
	}
	pod, err := m.sitter.GetPod(info.Namespace, info.Name)
	if err != nil {
		klog.Errorf("get pod %s failed: %s", info.Pod(), err.Error())
		return nil, err
	}
	klog.Infof("get pod %s/%s", pod.Namespace, pod.Name)

	// 2. check pod annotation
	if _, ok := pod.Annotations[schetypes.GPUAssume]; !ok {
		klog.Errorf("annotation %s does not on pod %s", schetypes.GPUAssume, info.Pod())
		return nil, fmt.Errorf("annotation %s does not on pod %s", schetypes.GPUAssume, info.Pod())
	}
	containerAssumedKey := fmt.Sprintf(schetypes.AnnotationGPUContainerOn, info.Container)
	val, ok := pod.Annotations[containerAssumedKey]
	if !ok {
		klog.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, info.Pod(), info.Container)
		return nil, fmt.Errorf("annotation %s does not on pod %s, container %s isn't assumed", containerAssumedKey, info.Pod(), info.Container)
	}

	// 3. get pod gpu index
	idx, err := strconv.Atoi(val)
	if err != nil {
		klog.Errorf("the %s assumed on pod %s, container %s may be not gpu index", val, info.Pod(), info.Container)
		return nil, fmt.Errorf("the %s assumed on pod %s, container %s may be not gpu index", val, info.Pod(), info.Container)
	}

	// 4. get the container resource
	var foundContainer *v1.Container

	for i, container := range pod.Spec.Containers {
		if container.Name != info.Container {
			continue
		}
		foundContainer = &pod.Spec.Containers[i]
	}

	if foundContainer != nil {
		// 5. create gpu unit and record it
		m.lock.Lock()
		defer m.lock.Unlock()

		gpuResources := v1.ResourceList{
			schetypes.ResourceGPUCore:   foundContainer.Resources.Requests[schetypes.ResourceGPUCore],
			schetypes.ResourceGPUMemory: foundContainer.Resources.Requests[schetypes.ResourceGPUMemory],
		}
		if err := m.operator.CreateGPUGrid(idx, gpuResources, devices); err != nil {
			klog.Errorf(err.Error())
			return nil, err
		}
		err = nil
		defer func() {
			if err != nil {
				klog.Error(err.Error())
				if deleteError := m.operator.DeleteGPUGrid(idx, devices); deleteError != nil {
					klog.Errorf("%s delete gpu unit failed: %s, delete reason: %s", info.String(), deleteError.Error(), err.Error())
				}
			}
		}()

		pif := m.storage.LoadOrCreate(info.Namespace, info.Name)
		pif.ContainerDevices[info.Container] = types.Device{
			GPU:       idx,
			Resources: gpuResources,
			List:      info.Devices,
		}
		if err = m.storage.Save(pif); err != nil {
			return nil, err
		}

		return &pluginapi.PreStartContainerResponse{}, err
	}

	return nil, fmt.Errorf("failed to find container %s in pod %s", info.Container, pod.Name)
}
