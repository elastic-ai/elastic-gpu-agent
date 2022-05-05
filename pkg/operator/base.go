package operator

import (
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"strconv"
)

type GPUOperator interface {
	Devices() ([]*Device, error)
	Create(index int, id string) error
	Delete(index int, id string) error
	Check(index int, id string) bool
}

type baseOperator struct {
}

func (g *baseOperator) Devices() (devices []*Device, err error) {
	err = g.devices(func(gpus []*nvml.Device) {
		for _, gpu := range gpus {
			dev := &Device{}
			uuid, ret := gpu.GetUUID()
			if ret != nvml.SUCCESS {
				err = fmt.Errorf("failed to get device UUID: %s", nvml.ErrorString(ret))
			}
			dev.UUID = uuid

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
			devices = append(devices, dev)
		}
	})

	return
}

func (b *baseOperator) devices(init func([]*nvml.Device)) (err error) {
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

type Device struct {
	UUID     string
	GPUIndex string
	Paths    []string
	// MB
	Memory uint64
}
