package manager

import (
	"github.com/nano-gpu/nano-gpu-agent/pkg/deviceplugins"
	"github.com/nano-gpu/nano-gpu-agent/pkg/gpu"
	"github.com/nano-gpu/nano-gpu-agent/pkg/kube"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	"sync"

	"github.com/nano-gpu/nano-gpu-agent/pkg/config"
	"github.com/nano-gpu/nano-gpu-agent/pkg/storage"

	"github.com/nano-gpu/nano-gpu-agent/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

type GPUManager interface {
	Run()
	GC()
	Restore() error
}

type GPUManagerImpl struct {
	nodeName        string
	dbPath          string
	sitter          kube.Sitter
	locator         kube.DeviceLocator
	storage         storage.Storage
	client          *kubernetes.Clientset
	gpuCorePlugin   *deviceplugins.Plugin
	gpuMemoryPlugin *deviceplugins.Plugin

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
		m.client = utils.MustNewClientInCluster()
	}
	if m.stopChan == nil {
		m.stopChan = config.NeverStop
	}
	metadb, err := storage.NewStorage(m.dbPath)
	if err != nil {
		return nil, err
	}
	m.sitter = kube.NewSitter(m.client, m.nodeName, m.GC)
	m.locator = kube.NewKubeletDeviceLocator(string(types.ResourceGPUCore))
	m.storage = metadb

	gpuOperator := gpu.NewGPUShare()
	gpuCoreDevicePlugin, err := deviceplugins.NewNanoGPUCoreDevicePlugin(m.locator, m.sitter, gpuOperator, m.storage)
	if err != nil {
		return nil, err
	}
	m.gpuCorePlugin = &deviceplugins.Plugin{
		Endpoint:           config.GPUCorePluginSock,
		ResourceName:       string(types.ResourceGPUCore),
		PreStartRequired:   true,
		DevicePluginServer: gpuCoreDevicePlugin,
	}

	gpuMemoryDevicePlugin, err := deviceplugins.NewNanoGPUMemoryDevicePlugin(gpuOperator)
	if err != nil {
		return nil, err
	}
	m.gpuMemoryPlugin = &deviceplugins.Plugin{
		Endpoint:           config.GPUMemoryPluginSock,
		ResourceName:       string(types.ResourceGPUMemory),
		PreStartRequired:   false,
		DevicePluginServer: gpuMemoryDevicePlugin,
	}
	return m, nil
}

func (m *GPUManagerImpl) Run() {
	go m.sitter.Start()
	go m.gc()
	go m.gpuCorePlugin.Run(m.stopChan)
	go m.gpuMemoryPlugin.Run(m.stopChan)
}

func (m *GPUManagerImpl) Restore() error {
	// TODO
	return nil
}

func (m *GPUManagerImpl) gc() {
	// TODO
}

func (m *GPUManagerImpl) GC() {
	select {
	case m.gcChan <- struct{}{}:
	default:
		return
	}
}
