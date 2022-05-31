package plugins

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/framework"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"elasticgpu.io/elastic-gpu/apis/elasticgpu/v1alpha1"
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"os"
	"strconv"
)

const (
	Path       = "/host/dev/"
	SymPath    = "/host/dev/elastic-gpu-%s"
	SymCtlPath = "/host/dev/elastic-gpuctl-%s"
)

func init() {
	framework.RegisterPlugin(&GPUSharePlugin{})
}

type GPUSharePlugin struct {
	Root    string
	RootCtl string
}

func (b *GPUSharePlugin) devices(init func([]*nvml.Device)) (err error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %s", nvml.ErrorString(ret))
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			err = fmt.Errorf("failed to shutdown NVML: %s", nvml.ErrorString(ret))
		}
	}()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to get device count: %s", nvml.ErrorString(ret))
	}

	var devs []*nvml.Device
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("failed to get device handle: %s", nvml.ErrorString(ret))
		}
		devs = append(devs, &device)
	}

	init(devs)
	return nil
}

func (g *GPUSharePlugin) Name() string {
	return "gpushare"
}

func (g *GPUSharePlugin) InterestedResources() []v1.ResourceName {
	return []v1.ResourceName{v1alpha1.ResourceGPUMemory}
}

func (g *GPUSharePlugin) List() []*framework.Device {
	gpuDevices := make([]*framework.Device, 0)
	var err error
	err = g.devices(func(gpus []*nvml.Device) {
		for _, gpu := range gpus {
			dev := &framework.Device{}
			uuid, ret := gpu.GetUUID()
			if ret != nvml.SUCCESS {
				err = fmt.Errorf("failed to get device UUID: %s", nvml.ErrorString(ret))
			}
			dev.ID = uuid

			index, ret := gpu.GetIndex()
			if ret != nvml.SUCCESS {
				err = fmt.Errorf("failed to get device index: %s", nvml.ErrorString(ret))
			}
			dev.GPUIndex = strconv.Itoa(index)

			memory, ret := gpu.GetMemoryInfo()
			if ret != nvml.SUCCESS {
				err = fmt.Errorf("failed to get device memory: %s", nvml.ErrorString(ret))
			}
			dev.Memory = memory.Total
			gpuDevices = append(gpuDevices, dev)
		}
	})
	if err != nil {
		klog.Errorf("fail to list devices in plugin %s: %v", g.Name(), err)
		return nil
	}

	devices := make([]*framework.Device, 0)
	for i := range gpuDevices {
		for j := uint64(0); j < common.GPUPercentEachCard; j++ {
			devices = append(devices, &framework.Device{
				ID:     fmt.Sprintf("%d-%02d", i, j),
				Health: "true",
			})
		}
	}
	return devices
}

func (g *GPUSharePlugin) Allocate(resource v1.ResourceName, devicesIDs []string) *framework.ContainerAllocate {
	device := types.NewDevice(devicesIDs, resource)
	faker := device.Hash

	deviceSpecs := []*framework.DeviceSpec{
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
			deviceSpecs = append(deviceSpecs, &framework.DeviceSpec{
				ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s-%d", faker, i),
				HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s-%d", faker, i),
				Permissions:   "rwm",
			})
		}
	} else {
		deviceSpecs = append(deviceSpecs, &framework.DeviceSpec{
			ContainerPath: fmt.Sprintf("/host/dev/elastic-gpu-%s-%d", faker, 0),
			HostPath:      fmt.Sprintf("/dev/elastic-gpu-%s-%d", faker, 0),
			Permissions:   "rwm",
		})
	}

	envs := map[string]string{
		"GPU":                    faker,
		"NVIDIA_VISIBLE_DEVICES": "none",
	}

	return &framework.ContainerAllocate{
		Envs:    envs,
		Devices: deviceSpecs,
	}
}

func (g *GPUSharePlugin) Create(index int, id string) error {
	devicePath := fmt.Sprintf(g.Root, index)
	deviceCtlPath := g.RootCtl

	symLink := fmt.Sprintf(SymPath, id)
	symCtlLink := fmt.Sprintf(SymCtlPath, id)

	if isExist(symLink) {
		return nil
	}
	err := os.Symlink(devicePath, symLink)
	if err != nil {
		klog.Errorf("Cannot create elastic gpu in this device")
		return err
	}
	if isExist(symCtlLink) {
		return nil
	}
	err = os.Symlink(deviceCtlPath, symCtlLink)
	if err != nil {
		klog.Errorf("Cannot create elastic gpuctl in this device")
		return err
	}
	return nil
}

func (g *GPUSharePlugin) Delete(index int, id string) error {
	symLink := fmt.Sprintf(SymPath, id)
	symCtlLink := fmt.Sprintf(SymCtlPath, id)
	err := os.Remove(symLink)
	if err != nil {
		return err
	}
	err = os.Remove(symCtlLink)
	if err != nil {
		return err
	}
	return nil
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}
