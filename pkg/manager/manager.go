package manager

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/framework"
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/storage"
	"elasticgpu.io/elastic-gpu/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"time"
)

type GPUManagerImpl struct {
	*framework.GPUPluginConfig
	kubeconf        string
	dbPath          string
	gpuPluginServer *GPUPluginServer

	stopChan chan struct{}
	gcChan   chan interface{}
}

type Option func(manager *GPUManagerImpl)

func WithNodeName(nodeName string) Option {
	return func(manager *GPUManagerImpl) {
		manager.NodeName = nodeName
	}
}

func WithKubeconf(kubeconf string) Option {
	return func(manager *GPUManagerImpl) {
		manager.kubeconf = kubeconf
	}
}

func WithDBPath(path string) Option {
	return func(manager *GPUManagerImpl) {
		manager.dbPath = path
	}
}

func WithGPUPluginName(gpuPluginName string) Option {
	return func(manager *GPUManagerImpl) {
		manager.GPUPluginName = gpuPluginName
	}
}

func NewGPUManager(options ...Option) (*GPUManagerImpl, error) {
	m := &GPUManagerImpl{
		gcChan:          make(chan interface{}, 1),
		GPUPluginConfig: &framework.GPUPluginConfig{},
	}
	for _, option := range options {
		option(m)
	}
	if m.kubeconf == "" {
		m.Client = common.MustNewClientInCluster()
		kubeconfig, err := restclient.InClusterConfig()
		if err != nil {
			return nil, err
		}
		eGPUClient, err := versioned.NewForConfig(kubeconfig)
		if err != nil {
			return nil, err
		}
		m.EGPUClient = eGPUClient
	} else {
		clientset, err := common.NewClientFromKubeconf(m.kubeconf)
		if err != nil {
			return nil, err
		}
		m.Client = clientset
		kubeconfig, err := clientcmd.BuildConfigFromFlags("", m.kubeconf)
		if err != nil {
			return nil, err
		}
		eGPUClient, err := versioned.NewForConfig(kubeconfig)
		if err != nil {
			return nil, err
		}
		m.EGPUClient = eGPUClient
	}

	if m.stopChan == nil {
		m.stopChan = common.NeverStop
	}
	metadb, err := storage.NewStorage(m.dbPath)
	if err != nil {
		return nil, err
	}
	m.Storage = metadb
	m.Sitter = kube.NewSitter(m.Client, m.NodeName, func(obj interface{}) {
		m.gcChan <- obj
	})

	m.DeviceLocator = make(map[v1.ResourceName]kube.DeviceLocator)

	gpuPluginServer, err := NewGPUPluginServer(m.GPUPluginConfig)
	if err != nil {
		return nil, err

	}
	m.gpuPluginServer = gpuPluginServer
	return m, nil
}

func (m *GPUManagerImpl) Run() {
	klog.Info("Start to run the manager.")
	go m.Sitter.Start()
	if err := wait.PollImmediateUntil(100*time.Millisecond, func() (bool, error) {
		synced := m.Sitter.HasSynced()
		return synced, nil
	}, m.stopChan); err != nil {
		klog.Fatalf("Fail to sync before run server: %v.", err)
	}

	if err := m.gpuPluginServer.Run(m.stopChan); err != nil {
		klog.Fatalf("Fail to run plugin server: %v.", err)
	}
	m.gpuPluginServer.GC(m.gcChan)
}
