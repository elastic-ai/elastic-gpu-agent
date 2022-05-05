package plugins

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"log"
	"net"
	"os"
	"path"
	"time"

	"github.com/fsnotify/fsnotify"

	"k8s.io/klog"

	"google.golang.org/grpc"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type Plugin struct {
	Endpoint           string
	ResourceName       string
	PreStartRequired   bool
	DevicePluginServer v1beta1.DevicePluginServer
}

func (p *Plugin) Run(stop <-chan struct{}) {
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

func (p *Plugin) Register() error {
	klog.Info("register")
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

func (p *Plugin) Serve(stop <-chan struct{}) {
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

func (p *Plugin) Wait() error {
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
