package plugins

import (
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type NvidiaDevicePlugin struct {
	baseDevicePlugin
}

func NewNvidiaDevicePlugin(config *GPUPluginConfig) (pluginapi.DevicePluginServer, error) {
	gpus, err := config.GPUOperator.Devices()
	if err != nil {
		return nil, err
	}
	devices := make([]*pluginapi.Device, 0)
	for _, g := range gpus {
		devices = append(devices, &pluginapi.Device{
			ID:     g.UUID,
			Health: pluginapi.Healthy,
		})
	}
	return &NvidiaDevicePlugin{
		baseDevicePlugin{GPUPluginConfig: config, devices: devices},
	}, nil
}

//func (c *NvidiaDevicePlugin) discoveryEGPU(devices []*operator.Device) error {
//	for _, d := range devices {
//		elasticGPU := &v1alpha1.ElasticGPU{}
//		elasticGPU.Name = fmt.Sprintf("pgpu-%s", strings.ToLower(d.ID))
//		elasticGPU.Spec.Capacity = v1alpha1.ResourceList{
//			v1alpha1.ResourcePGPU: resource.MustParse("1"),
//		}
//		elasticGPU.Spec.ElasticGPUSource = v1alpha1.ElasticGPUSource{
//			PhysicalGPU: &v1alpha1.PhysicalGPU{
//				GPUIndex: d.GPUIndex,
//				GPUUUID:  d.ID,
//			},
//		}
//		elasticGPU.Status = v1alpha1.ElasticGPUStatus{
//			Phase: v1alpha1.GPUAvailable,
//		}
//		elasticGPU.Spec.NodeName = c.NodeName
//		_, err := c.EGPUClient.ElasticgpuV1alpha1().ElasticGPUs("default").Create(context.TODO(), elasticGPU, v1meta.CreateOptions{})
//		if err != nil {
//			if strings.Contains(err.Error(), "already exists") {
//				continue
//			}
//			return err
//		}
//	}
//
//	return nil
//}
//
//func (c *NvidiaDevicePlugin) init() (*NvidiaDevicePlugin, error) {
//	devices, err := c.GPUOperator.Devices()
//	if err != nil {
//		return nil, err
//	}
//
//	//err = c.discoveryEGPU(devices)
//	//if err != nil {
//	//	return nil, err
//	//}
//
//	pluginDevices := make([]*pluginapi.Device, 0, len(devices))
//	for _, d := range devices {
//		pluginDevices = append(pluginDevices, &d.Device)
//	}
//	c.devices = pluginDevices
//
//	return c, err
//}
//
//func (c *NvidiaDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
//	return &pluginapi.DevicePluginOptions{
//		PreStartRequired: true,
//	}, nil
//}
//
//func (c *NvidiaDevicePlugin) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
//	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: c.devices}); err != nil {
//		klog.Errorf("failed to send response: %v", err)
//		return err
//	}
//	<-server.Context().Done()
//	return nil
//}
//
//func (c *NvidiaDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
//	return &pluginapi.AllocateResponse{
//		ContainerResponses: []*pluginapi.ContainerAllocateResponse{{
//			Envs: map[string]string{
//				"NVIDIA_VISIBLE_DEVICES": strings.Join(request.ContainerRequests[0].DevicesIDs, ","),
//			},
//		}},
//	}, nil
//}
//
//func (c *NvidiaDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
//	devices := types.NewDevice(request.DevicesIDs)
//	info, err := c.DeviceLocator.Locate(devices)
//	if err != nil {
//		klog.Errorf("get pod %s failed: %s", info.Pod(), err.Error())
//		return nil, err
//	}
//	pod, err := c.Client.CoreV1().Pods(info.Namespace).Get(context.TODO(), info.Name, v1meta.GetOptions{})
//	if err != nil {
//		klog.Errorf("get pod %s failed: %s", info.Pod(), err.Error())
//		return nil, err
//	}
//	of := v1.ObjectReference{
//		APIVersion: pod.APIVersion,
//		Kind:       pod.Kind,
//		Name:       pod.Name,
//		UID:        pod.UID,
//		FieldPath:  fmt.Sprintf("spec.containers{%s}", info.Container),
//	}
//	for _, id := range request.DevicesIDs {
//		ename := fmt.Sprintf("pgpu-%s", strings.ToLower(id))
//		egpu := &v1alpha1.ElasticGPU{
//			Spec: v1alpha1.ElasticGPUSpec{
//				ClaimRef: of,
//			},
//			Status: v1alpha1.ElasticGPUStatus{Phase: v1alpha1.GPUBound},
//		}
//		egpu.Name = ename
//		_, err := c.EGPUClient.ElasticgpuV1alpha1().ElasticGPUs(info.Namespace).Update(context.TODO(), egpu, v1meta.UpdateOptions{})
//		if err != nil {
//			klog.Errorf("failed to update elasticgpu: %s, %v", ename, err)
//			continue
//		}
//	}
//	return &pluginapi.PreStartContainerResponse{}, nil
//}
