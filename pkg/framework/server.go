package framework

import (
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"elasticgpu.io/elastic-gpu-agent/pkg/kube"
	"elasticgpu.io/elastic-gpu-agent/pkg/types"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"k8s.io/klog"
)

type GPUPluginServer struct {
	*GPUPluginConfig
	pluginServers map[v1.ResourceName]*DevicePluginServer
	plugin        GPUPlugin
}

func NewGPUPluginServer(c *GPUPluginConfig) (*GPUPluginServer, error) {
	gps := &GPUPluginServer{GPUPluginConfig: c, pluginServers: make(map[v1.ResourceName]*DevicePluginServer)}
	plugin, ok := registeredPlugins[c.GPUPluginName]
	if !ok {
		return nil, fmt.Errorf("cannot find plugin %s", c.GPUPluginName)
	}
	gps.plugin = plugin

	for _, r := range plugin.InterestedResources() {
		c.DeviceLocator[r] = kube.NewKubeletDeviceLocator(string(r))
		gps.pluginServers[r] = &DevicePluginServer{
			// TODO: Maybe a short name?
			Endpoint:           fmt.Sprintf("%s.sock", string(r)),
			ResourceName:       string(r),
			PreStartRequired:   true,
			DevicePluginServer: NewDevicePlugin(c, r, gps.plugin),
		}
	}

	return gps, nil
}

func (g *GPUPluginServer) Run(stop <-chan struct{}) error {
	if g.pluginServers == nil && len(g.pluginServers) == 0 {
		return fmt.Errorf("gpu device plugins is empty")
	}

	for k, v := range g.pluginServers {
		klog.Infof("start plugin server %s", k)
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
			klog.Info("plugin %s starts to GC")
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
						klog.Errorf("get pods %s/%s failed: %s", info.Namespace, info.Name)
					}
				}
				return nil
			})
			if err != nil {
				klog.Error("iterate pod failed: %s", err.Error())
			}
			for _, l := range devicesToDelete {
				if err = g.plugin.Delete(common.UselessNumber, fmt.Sprintf("%s-%d", l.device.Hash, 0)); err != nil {
					break
				}

				if err := g.Storage.Delete(l.namespace, l.name); err != nil {
					klog.Errorf("delete elastic gpu record for %s %s %s failed: %s", l.namespace, l.name, l.container, err.Error())
					continue
				}
			}
		}
	}()
}
