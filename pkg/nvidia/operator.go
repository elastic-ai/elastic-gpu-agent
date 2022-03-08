package nvidia

import (
	"fmt"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"regexp"

	//"tkestack.io/nvml"
	//"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

const (
	GPUPath = "/dev/nvidia%d"
	GPUCtlPath = "/dev/nvidiactl"
	Path = "/host/dev/"
	GPUNamePattern = "nvidia"
	SymPath = "/host/dev/nano-gpu-%s"
	SymCtlPath = "/host/dev/nano-gpuctl-%s"
)

type GPUOperator interface {
	DetectNumber() (int, error)
	Create(index int, id string) error
	Delete(index int, id string) error
	Check(index int, id string) bool
}

var (
	validGPUName = regexp.MustCompile(GPUNamePattern + `\d+`)
)

func IsNvidiaGPU(id string) bool {
	return validGPUName.MatchString(id)
}

// TODO: We should implement GPUShareOperator here.
// We should make a soft link for the used nvidia device.
// Docker will handle the link and set the device cgroup.
// See: https://github.com/moby/moby/blob/5176095455642c30642efacf6f35afc7c6dede92/oci/devices_linux.go#L41

type GPUShareOperator struct {
	Root string
	RootCtl string
}

func NewGPUOperator() GPUOperator {
	return &GPUShareOperator{
		Root: GPUPath,
		RootCtl:GPUCtlPath,
	}
}

func (G *GPUShareOperator) DetectNumber() (int, error) {
	files, err := ioutil.ReadDir(Path)
	if err != nil {
		klog.Error(err)
	}
	var count int
	for _, file := range files {
		if IsNvidiaGPU(file.Name()) {
			count ++
		}
	}
	//count, err := nvml.GetDeviceCount()
	//count, err := nvml.DeviceGetCount()
	//klog.Info("count:",count)
	//if err != nil {
	//	klog.Errorf("Cannot find any GPU in this device")
	//}
	return count, nil
}

func (G *GPUShareOperator) Create(index int, id string) error {
	devicePath := fmt.Sprintf(G.Root, index)
	deviceCtlPath := G.RootCtl

	symLink := fmt.Sprintf(SymPath, id)
	symCtlLink := fmt.Sprintf(SymCtlPath, id)

	if IsExist(symLink){
		return nil
	}
	err := os.Symlink(devicePath, symLink)
	if err != nil {
		klog.Errorf("Cannot create nano gpu in this device")
		return err
	}
	if IsExist(symCtlLink){
		return nil
	}
	err = os.Symlink(deviceCtlPath, symCtlLink)
	if err != nil {
		klog.Errorf("Cannot create nano gpuctlin this device")
		return err
	}
	return nil
}

func (G *GPUShareOperator) Delete(index int, id string) error {
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

func (G *GPUShareOperator) Check(index int, id string) bool {
	symLink := fmt.Sprintf(SymPath, id)
	_, err1 := os.Stat(symLink)
	symCtlLink := fmt.Sprintf(SymCtlPath, id)
	_, err2 := os.Stat(symCtlLink)
	return err1 == nil && err2 == nil
}

func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}
