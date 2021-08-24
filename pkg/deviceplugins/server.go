package deviceplugins

import (
	"context"
	"k8s.io/klog"
	"net"
	"os"
	"path"
	"time"

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
	p.Serve(stop)
	if err := p.Wait(); err != nil {
		klog.Error(err.Error())
	}
	klog.Info("start register")
	time.Sleep(time.Second * 3)
	if err := p.Register(); err != nil {
		klog.Error(err.Error())
	}
	klog.Info("finish register")
}

func (p *Plugin) Register() error {
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
		klog.Info("plugin %s exit", p.ResourceName)
	}()

	go func() {
		<-stop
		server.GracefulStop()
		listener.Close()
	}()
}

func (p *Plugin) Wait() error {
	conn, err := grpc.Dial(path.Join(v1beta1.DevicePluginPath, p.Endpoint), grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(time.Second * 5),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),)
	if err != nil {
		return err
	}
	return conn.Close()
}
