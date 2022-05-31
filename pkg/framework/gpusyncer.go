package framework

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/operator"
	"elasticgpu.io/elastic-gpu/apis/elasticgpu/v1alpha1"
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"reflect"
)

const (
	labelNode = "elasticgpu.io/node"
)

type GPUSyncer struct {
	*GPUPluginConfig
	gpu *operator.PhyGPUOperator
}

func NewGPUSyncer(plugin *GPUPluginConfig) *GPUSyncer {
	return &GPUSyncer{gpu: &operator.PhyGPUOperator{}, GPUPluginConfig: plugin}
}

func (g *GPUSyncer) Sync() error {
	gpuMaps := make(map[string]*v1alpha1.GPU)
	err := g.gpu.ListDevices(func(devices []*nvml.Device) {
		for _, device := range devices {
			uuid, ret := device.GetUUID()
			if ret != nvml.SUCCESS {
				klog.Errorf("Unable to get device uuid: %v", nvml.ErrorString(ret))
			}
			index, ret := device.GetIndex()
			if ret != nvml.SUCCESS {
				klog.Errorf("Unable to get device index: %v", nvml.ErrorString(ret))
			}
			name, ret := device.GetName()
			if ret != nvml.SUCCESS {
				klog.Errorf("Unable to get device name: %v", nvml.ErrorString(ret))
			}
			memory, ret := device.GetMemoryInfo()
			if ret != nvml.SUCCESS {
				klog.Errorf("Unable to get memory info: %v", nvml.ErrorString(ret))
			}
			minor, ret := device.GetMinorNumber()
			if ret != nvml.SUCCESS {
				klog.Errorf("Unable to get minor number: %v", nvml.ErrorString(ret))
			}
			gpuMaps[uuid] = &v1alpha1.GPU{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%02d", g.NodeName, index),
					Labels: map[string]string{
						labelNode: g.NodeName,
					},
				},
				Spec: v1alpha1.GPUSpec{
					Index:    index,
					UUID:     uuid,
					Model:    name,
					Path:     fmt.Sprintf("/dev/nvidia%d", minor),
					Memory:   memory.Total,
					NodeName: g.NodeName,
				},
			}
		}
	})
	if err != nil {
		return err
	}

	gpuList, err := g.EGPUClient.ElasticgpuV1alpha1().GPUs().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", labelNode, g.NodeName),
	})
	if err != nil {
		return err
	}

	createFunc := func(gpu *v1alpha1.GPU) error {
		_, err = g.EGPUClient.ElasticgpuV1alpha1().GPUs().Create(context.TODO(), gpu, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	}
	updateFunc := func(gpu *v1alpha1.GPU) error {
		_, err = g.EGPUClient.ElasticgpuV1alpha1().GPUs().Update(context.TODO(), gpu, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}

	deleteFunc := func(gpu *v1alpha1.GPU) error {
		err = g.EGPUClient.ElasticgpuV1alpha1().GPUs().Delete(context.TODO(), gpu.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		return nil
	}

	needUpdate := make([]*v1alpha1.GPU, 0)
	needDelete := make([]*v1alpha1.GPU, 0)
	for i, o := range gpuList.Items {
		n, ok := gpuMaps[o.Spec.UUID]
		if !ok {
			needDelete = append(needDelete, &gpuList.Items[i])
			continue
		}
		if !reflect.DeepEqual(o.Spec, n.Spec) {
			needUpdate = append(needUpdate, gpuMaps[o.Spec.UUID])
		}
		delete(gpuMaps, o.Spec.UUID)
	}

	for i := range needDelete {
		if err := deleteFunc(needDelete[i]); err != nil {
			klog.Errorf("Fail to delete gpu crd %s on node %s: %v.", needDelete[i].Name, g.NodeName, err)
		}
	}
	for i := range needUpdate {
		if err := updateFunc(needUpdate[i]); err != nil {
			klog.Errorf("Fail to update gpu crd %s on node %s: %v.", needUpdate[i].Name, g.NodeName, err)
		}
	}
	for i := range gpuMaps {
		if err := createFunc(gpuMaps[i]); err != nil {
			klog.Errorf("Fail to create gpu crd %s on node %s: %v.", gpuMaps[i].Name, g.NodeName, err)
		}
	}

	return nil
}
