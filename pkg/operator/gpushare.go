package operator

import (
	"fmt"
	"k8s.io/klog"
	"os"
)

const (
	GPUPath        = "/dev/nvidia%d"
	GPUCtlPath     = "/dev/nvidiactl"
	Path           = "/host/dev/"
	GPUNamePattern = "nvidia"
	SymPath        = "/host/dev/elastic-gpu-%s"
	SymCtlPath     = "/host/dev/elastic-gpuctl-%s"
)

type GPUShareOperator struct {
	baseOperator
	Root    string
	RootCtl string
}

func NewGPUShareOperator() GPUOperator {
	return &GPUShareOperator{
		Root:    GPUPath,
		RootCtl: GPUCtlPath,
	}
}

func (G *GPUShareOperator) Create(index int, id string) error {
	devicePath := fmt.Sprintf(G.Root, index)
	deviceCtlPath := G.RootCtl

	symLink := fmt.Sprintf(SymPath, id)
	symCtlLink := fmt.Sprintf(SymCtlPath, id)

	if IsExist(symLink) {
		return nil
	}
	err := os.Symlink(devicePath, symLink)
	if err != nil {
		klog.Errorf("Cannot create elastic gpu in this device")
		return err
	}
	if IsExist(symCtlLink) {
		return nil
	}
	err = os.Symlink(deviceCtlPath, symCtlLink)
	if err != nil {
		klog.Errorf("Cannot create elastic gpuctl in this device")
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
