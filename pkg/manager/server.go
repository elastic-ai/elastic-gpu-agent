package manager

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/framework"
	_ "elasticgpu.io/elastic-gpu-agent/pkg/framework/plugins"
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"time"

	"k8s.io/klog"
)

type GPUPluginServer struct {
	*framework.GPUPluginConfig
	pluginServers map[v1.ResourceName]*framework.DevicePluginServer
	plugin        framework.GPUPlugin
}

func NewGPUPluginServer(c *framework.GPUPluginConfig) (*GPUPluginServer, error) {
	gps := &GPUPluginServer{GPUPluginConfig: c, pluginServers: make(map[v1.ResourceName]*framework.DevicePluginServer)}
	plugin, ok := framework.RegisteredPlugins[c.GPUPluginName]
	if !ok {
		return nil, fmt.Errorf("cannot find plugin %s", c.GPUPluginName)
	}
	gps.plugin = plugin

	for _, r := range plugin.InterestedResources() {
		c.DeviceLocator[r] = kube.NewKubeletDeviceLocator(string(r))
		gps.pluginServers[r] = &framework.DevicePluginServer{
			// TODO: Maybe a short name?
			Endpoint:           fmt.Sprintf("%s.sock", strings.Replace(string(r), "/", "-", -1)),
			ResourceName:       string(r),
			PreStartRequired:   true,
			DevicePluginServer: framework.NewDevicePlugin(c, r, gps.plugin),
		}
	}

	return gps, nil
}

func (g *GPUPluginServer) Run(stop <-chan struct{}) error {
	if g.pluginServers == nil && len(g.pluginServers) == 0 {
		return fmt.Errorf("gpu device plugins is empty")
	}

	for k, v := range g.pluginServers {
		klog.Infof("Start to run plugin server %s.", k)
		go v.Run(stop)
	}

	return nil
}

func (g *GPUPluginServer) GC(gcChan <-chan interface{}) {
	go func() {
		for {
			select {
			case o := <-gcChan:
				if pod, ok := o.(v1.Pod); ok && pod.Annotations[common.ElasticGPUAssumedAnnotation] != "true" {
					continue
				}
			case <-time.After(time.Minute):
			}
			klog.Infof("Plugin %s starts to GC.", g.plugin.Name())
			type line struct {
				namespace string
				name      string
				container string
				device    *types.Device
			}

			devicesToDelete := []line{}
			err := g.Storage.ForEach(func(info *types.PodInfo) error {
				_, err := g.Sitter.GetPod(info.Namespace, info.Name)
				if err != nil {
					_, apiError := g.Sitter.GetPodFromApiServer(info.Namespace, info.Name)
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
						klog.Errorf("Fail to get pod %s/%s: %v.", info.Namespace, info.Name, err)
					}
				}
				return nil
			})
			if err != nil {
				klog.Errorf("Fail to iterate pod: %v.", err)
			}
			for _, l := range devicesToDelete {
				if err = g.plugin.Delete(common.UselessNumber, fmt.Sprintf("%s-%d", l.device.Hash, 0)); err != nil {
					break
				}

				if err := g.Storage.Delete(l.namespace, l.name); err != nil {
					klog.Errorf("Fail to delete gpu device %s/%s on container %s: %v.", l.namespace, l.name, l.container, err)
					continue
				}
			}
		}
	}()
}
