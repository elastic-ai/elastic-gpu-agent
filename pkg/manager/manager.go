package manager

import (
	"fmt"
	"manager/pkg/common"
	"manager/pkg/kube"
	"manager/pkg/nvidia"
	"manager/pkg/plugins"
	"manager/pkg/storage"
	"manager/pkg/types"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type GPUManager interface {
	Run()
	GC()
	Restore() error
}

type GPUManagerImpl struct {
	nodeName string
	dbPath   string
	sitter   kube.Sitter
	locator  kube.DeviceLocator
	storage  storage.Storage
	client   *kubernetes.Clientset

	operator nvidia.GPUOperator
	plugin  *plugins.Plugin

	stopChan chan struct{}
	gcOnce   sync.Once
	gcChan   chan struct{}
}

type Option func(manager *GPUManagerImpl)

func WithNodeName(nodeName string) Option {
	return func(manager *GPUManagerImpl) {
		manager.nodeName = nodeName
	}
}

func WithClientset(client *kubernetes.Clientset) Option {
	return func(manager *GPUManagerImpl) {
		manager.client = client
	}
}

func WithDBPath(path string) Option {
	return func(manager *GPUManagerImpl) {
		manager.dbPath = path
	}
}

func WithStopChan(ch chan struct{}) Option {
	return func(manager *GPUManagerImpl) {
		manager.stopChan = ch
	}
}

func NewGPUManager(options ...Option) (GPUManager, error) {
	m := &GPUManagerImpl{
		gcChan: make(chan struct{}, 1),
	}
	for _, option := range options {
		option(m)
	}
	if m.client == nil {
		m.client = common.MustNewClientInCluster()
	}
	if m.stopChan == nil {
		m.stopChan = common.NeverStop
	}
	metadb, err := storage.NewStorage(m.dbPath)
	if err != nil {
		return nil, err
	}
	m.sitter = kube.NewSitter(m.client, m.nodeName, m.GC)
	m.locator = kube.NewKubeletDeviceLocator(common.ResourceName)
	m.operator = nvidia.NewGPUOperator()
	m.storage = metadb

	coreDevicePlugin, err := plugins.NewNanoServer(m.locator, m.sitter, m.operator, m.storage)
	if err != nil {
		return nil, err
	}
	m.plugin = &plugins.Plugin{
		Endpoint:           common.NanoGPUSock,
		ResourceName:       common.ResourceName,
		PreStartRequired:   true,
		DevicePluginServer: coreDevicePlugin,
	}
	return m, nil
}

func (m *GPUManagerImpl) Run() {
	go m.sitter.Start()
	go m.gc()
	go m.plugin.Run(m.stopChan)
}

func (m *GPUManagerImpl) Restore() error {
	return m.storage.ForEach(func(pi *types.PodInfo) error {
		for container, device := range pi.ContainerDeviceMap {
			pod, err := m.sitter.GetPod(pi.Namespace, pi.Name)
			if err != nil {
				return err
			}
			annotationKey := fmt.Sprintf(common.NanoGPUContainerAnnotation, container)
			val, ok := pod.Annotations[annotationKey]
			if !ok {
				return fmt.Errorf("annotation %s does not on pod %s, container %s isn't assumed", annotationKey, pi, container)
			}
			idx, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("the %s assumed on pod %s, container %s may be not qgpu index", val, pi, container)
			}
			if !m.operator.Check(idx, device.Hash) {
				if err := m.operator.Create(idx, device.Hash); err != nil {
					klog.Errorf("operator create failed: %s", err.Error())
				}
			}
		}
		return nil
	})
}

func (m *GPUManagerImpl) gc() {
	for {
		// gc here
		klog.Info("start gc")
		type line struct {
			namespace string
			name      string
			container string
			device    *types.Device
		}

		devicesToDelete := []line{}
		err := m.storage.ForEach(func(info *types.PodInfo) error {
			_, err := m.sitter.GetPod(info.Namespace, info.Name)
			if err != nil {
				_, apiError := m.sitter.GetPodFromApiServer(info.Namespace, info.Name)
				if errors.IsNotFound(apiError) {
					for name, device := range info.ContainerDeviceMap {
						devicesToDelete = append(devicesToDelete, line{
							namespace: info.Namespace,
							name:      info.Name,
							container: name,
							device:    device,
						})
					}
				} else {
					klog.Errorf("get pods %s/%s failed: %s", info.Namespace, info.Name)
				}
			}
			return nil
		})
		if err != nil {
			klog.Error("iterate pod failed: %s", err.Error())
		}
		for _, l := range devicesToDelete {
			if err := m.operator.Delete(common.UselessNumber, l.device.Hash); err != nil {
				klog.Errorf("delete qgpu for %s %s %s failed: %s", l.namespace, l.name, l.container, err.Error())
				continue
			}
			if err := m.storage.Delete(l.namespace, l.name); err != nil {
				klog.Errorf("delete qgpu record for %s %s %s failed: %s", l.namespace, l.name, l.container, err.Error())
				continue
			}
		}
		select {
		case <-m.gcChan:
		case <-time.After(time.Minute):
		}
	}
}

func (m *GPUManagerImpl) GC() {
	select {
	case m.gcChan <- struct{}{}:
	default:
		return
	}
}
