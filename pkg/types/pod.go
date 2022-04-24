package types

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type PodContainer struct {
	Namespace string
	Name      string
	Container string
}

func (pc *PodContainer) String() string {
	return fmt.Sprintf("%s/%s:%s", pc.Namespace, pc.Name, pc.Container)
}

func (pc *PodContainer) Pod() string {
	return fmt.Sprintf("%s/%s", pc.Namespace, pc.Name)
}

type PodInfo struct {
	Namespace string
	Name      string

	ContainerDeviceMap map[string]*Device
}

func NewPI(namespace, name string) *PodInfo {
	return &PodInfo{
		Namespace:          namespace,
		Name:               name,
		ContainerDeviceMap: map[string]*Device{},
	}
}

func NewPIFromRaw(key, val []byte) (*PodInfo, error) {
	arr := strings.Split(string(key), "/")
	if len(arr) != 2 {
		return nil, fmt.Errorf("error key format: %s", string(key))
	}
	pi := NewPI(arr[0], arr[1])
	if err := json.Unmarshal(val, &pi.ContainerDeviceMap); err != nil {
		return nil, fmt.Errorf("error val format: %s", err.Error())
	}
	return pi, nil
}

func (i *PodInfo) Key() []byte {
	return []byte(path.Join(i.Namespace, i.Name))
}

func (i *PodInfo) Val() []byte {
	bs, _ := json.Marshal(i.ContainerDeviceMap)
	return bs
}

func (i *PodInfo) SetVal(val []byte) error {
	return json.Unmarshal(val, &i.ContainerDeviceMap)
}
