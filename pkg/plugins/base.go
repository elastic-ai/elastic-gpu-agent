package plugins

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/operator"
	"elasticgpu.io/elastic-gpu-agent/pkg/storage"
	"elasticgpu.io/elastic-gpu/api/v1alpha1"
	"elasticgpu.io/elastic-gpu/clientset/versioned"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"log"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"k8s.io/klog"

	"google.golang.org/grpc"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type GPUPluginConfig struct {
	DeviceLocator kube.DeviceLocator
	Sitter        kube.Sitter
	Storage       storage.Storage
	GPUPluginName GPUPluginName
	Client        *kubernetes.Clientset
	EGPUClient    *versioned.Clientset
	GPUOperator   operator.GPUOperator
	NodeName      string
	IsPrivateMode bool
	LSEndpoint    string
}

type GPUPluginName string

const (
	GPUSHARE GPUPluginName = "gpushare"
	//QGPU     GPUPluginName = "qgpu"
)

func PluginFactory(dpc *GPUPluginConfig) (GPUPlugin, error) {
	switch dpc.GPUPluginName {
	case GPUSHARE:
		dpc.GPUOperator = operator.NewGPUShareOperator()
		dpc.DeviceLocator = kube.NewKubeletDeviceLocator(string(v1alpha1.ResourceGPUMemory))
		return NewGPUSharePlugin(dpc)
	}
	return nil, fmt.Errorf("cannot find plugin %s", dpc.GPUPluginName)
}

type baseDevicePlugin struct {
	PreStartRequired bool
	devices          []*pluginapi.Device
	*GPUPluginConfig
	lock sync.Mutex
}

func (c *baseDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

func (c *baseDevicePlugin) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := server.Send(&pluginapi.ListAndWatchResponse{Devices: c.devices}); err != nil {
		return err
	}
	<-server.Context().Done()
	return nil
}

func (m *baseDevicePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return &pluginapi.AllocateResponse{ContainerResponses: make([]*pluginapi.ContainerAllocateResponse, 0)}, nil
}

func (m *baseDevicePlugin) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (c *baseDevicePlugin) GetPreferredAllocation(ctx context.Context, request *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

type DevicePluginServer struct {
	Endpoint           string
	ResourceName       string
	PreStartRequired   bool
	DevicePluginServer v1beta1.DevicePluginServer
}

func (p *DevicePluginServer) Run(stop <-chan struct{}) {
	errChan := make(chan error, 1)
	stoChan := make(chan struct{})
	watcher, err := common.NewFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		klog.Fatalf("create fswatch failed: %s", err.Error())
	}
restart:
	close(stoChan)
	time.Sleep(time.Second)
	stoChan = make(chan struct{})
	p.Serve(stoChan)
	if err := p.Wait(); err != nil {
		klog.Error(err.Error())
		goto restart
	}
	if err := p.Register(); err != nil {
		errChan <- err
	}
	for {
		select {
		case err := <-errChan:
			klog.Errorf("register error: %s", err.Error())
			goto restart
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				goto restart
			}
		case <-stop:
			close(stoChan)
			return
		}
	}
}

func (p *DevicePluginServer) Register() error {
	conn, err := grpc.Dial(v1beta1.KubeletSocket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = v1beta1.NewRegistrationClient(conn).Register(context.Background(), &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     p.Endpoint,
		ResourceName: p.ResourceName,
		Options: &pluginapi.DevicePluginOptions{
			PreStartRequired: p.PreStartRequired,
		},
	})
	return err
}

func (p *DevicePluginServer) Serve(stop <-chan struct{}) {
	_ = os.Remove(path.Join(v1beta1.DevicePluginPath, p.Endpoint))
	listener, err := net.Listen("unix", path.Join(v1beta1.DevicePluginPath, p.Endpoint))
	if err != nil {
		panic(err)
	}

	server := grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(server, p.DevicePluginServer)
	go func() {
		if err := server.Serve(listener); err != nil {
			panic(err)
		}
		klog.Infof("plugin %s exit", p.ResourceName)
	}()

	go func() {
		<-stop
		server.GracefulStop()
		listener.Close()
	}()
}

func (p *DevicePluginServer) Wait() error {
	time.Sleep(time.Second)
	conn, err := grpc.Dial(path.Join(v1beta1.DevicePluginPath, p.Endpoint), grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(time.Second*5),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return err
	}
	return conn.Close()
}

type GPUPlugin interface {
	Run(stop <-chan struct{})
	GC(gcChan <-chan interface{})
}

type GPUSharePlugin struct {
	plugins map[string]*DevicePluginServer
	*GPUPluginConfig
}

func NewGPUSharePlugin(c *GPUPluginConfig) (GPUPlugin, error) {
	gp := &GPUSharePlugin{GPUPluginConfig: c, plugins: make(map[string]*DevicePluginServer)}
	dp, err := NewGPUShareDevicePlugin(c)
	if err != nil {
		return nil, err
	}
	gp.plugins[string(v1alpha1.ResourceGPUMemory)] = &DevicePluginServer{
		Endpoint:           "elastic-gpushare.sock",
		ResourceName:       string(v1alpha1.ResourceGPUMemory),
		PreStartRequired:   true,
		DevicePluginServer: dp,
	}

	dpCore, err := NewGPUShareCoreDevicePlugin(c)
	if err != nil {
		return nil, err
	}
	gp.plugins[string(v1alpha1.ResourceGPUCore)] = &DevicePluginServer{
		Endpoint:           "elastic-gpushare-core.sock",
		ResourceName:       string(v1alpha1.ResourceGPUCore),
		PreStartRequired:   true,
		DevicePluginServer: dpCore,
	}

	return gp, nil
}

func (g *GPUSharePlugin) Run(stop <-chan struct{}) {
	for k, _ := range g.plugins {
		go g.plugins[k].Run(stop)
	}
}
func (g *GPUSharePlugin) GC(gcChan <-chan interface{}) {

}
