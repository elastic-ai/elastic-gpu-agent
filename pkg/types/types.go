package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/klog"
	"sort"
	"strings"
)

type ResourceInfo struct {
	Namespace string
	Name      string
	Container string
	Devices   DeviceList
}

func (i ResourceInfo) String() string {
	return fmt.Sprintf("namespace: %s, pod: %s, container: %s, device hash: %s", i.Namespace, i.Name, i.Container, i.Devices.Hash())
}

func (i ResourceInfo) Pod() string {
	return fmt.Sprintf("namespace: %s, pod: %s", i.Namespace, i.Name)
}

type PodInfo struct {
	Namespace        string
	Name             string
	ContainerDevices map[string]Device
}

type Device struct {
	GPU       int
	Resources v1.ResourceList
	List      DeviceList
}

func NewPodInfo(namespace, name string) *PodInfo {
	return &PodInfo{
		Namespace:        namespace,
		Name:             name,
		ContainerDevices: map[string]Device{},
	}
}

func (p *PodInfo) Key() string {
	return fmt.Sprintf("%s:%s", p.Namespace, p.Name)
}

func (p *PodInfo) ToBytes() []byte {
	buffer := &bytes.Buffer{}
	_ = json.NewEncoder(buffer).Encode(p)
	return buffer.Bytes()
}

func (p *PodInfo) FromBytes(buffer []byte) {
	buf := bytes.NewBuffer(buffer)
	_ = json.NewDecoder(buf).Decode(p)
}

type DeviceList []string

func (dl DeviceList) Equals(ol DeviceList) bool {
	defer klog.Info("call device equals")
	if len(dl) != len(ol) {
		klog.Errorf("length not equals %d %d", len(dl), len(ol))
		return false
	}
	sort.Strings(dl)
	sort.Strings(ol)
	for i := 0; i < len(dl); i++ {
		if dl[i] != ol[i] {
			klog.Errorf("element not equals\n %v\n %v\n", dl, ol)
			return false
		}
	}
	return true
}

func (dl DeviceList) Hash() string {
	return hex.EncodeToString(to(sha256.Sum256([]byte(strings.Join(dl, ":")))))[0:8]
}

func to(bs [32]byte) []byte {
	return bs[0:32]
}
