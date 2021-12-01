package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"

	"manager/pkg/podresources"
	"manager/pkg/podresources/v1alpha1"
	"manager/pkg/types"
)

type DeviceLocator interface {
	Locate(devices *types.Device) (*types.PodContainer, error)
	List() ([]*types.PodInfo, error)
	Close() error
}

type KubeletDeviceLocator struct {
	err      error
	resource string
	client   v1alpha1.PodResourcesListerClient
	conn     *grpc.ClientConn
	lock     sync.Mutex
}

func NewKubeletDeviceLocator(resource string) DeviceLocator {
	ep, _ := podresources.LocalEndpoint(podresources.PodResourceRoot, podresources.Socket)
	client, conn, err := podresources.GetClient(ep, 10*time.Second, 1024*1024*16)
	return &KubeletDeviceLocator{
		resource: resource,
		client:   client,
		conn:     conn,
		err:      err,
	}
}

func (k *KubeletDeviceLocator) Locate(devices *types.Device) (*types.PodContainer, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	if k.err != nil {
		ep, _ := podresources.LocalEndpoint(podresources.PodResourceRoot, podresources.Socket)
		k.client, k.conn, k.err = podresources.GetClient(ep, 10*time.Second, 1024*1024*16)
		if k.err != nil {
			return nil, k.err
		}
	}

	response, err := k.client.List(context.Background(), &v1alpha1.ListPodResourcesRequest{})
	if err != nil {
		k.err = err
		return nil, err
	}
	// pod -> container -> resource
	for _, pod := range response.PodResources {
		for _, container := range pod.Containers {
			deviceIds := []string{}
			for _, resource := range container.Devices {
				if resource.ResourceName == k.resource {
					// for k8s 1.20-, resource.DeviceIds contain all device IDs
					if devices.Equals(types.NewDevice(resource.DeviceIds)) {
						klog.Infof("pod %s/%s lodated with device list %v", pod.Namespace, pod.Name, resource.DeviceIds)
						return &types.PodContainer{
							Namespace: pod.Namespace,
							Name:      pod.Name,
							Container: container.Name,
						}, nil
					} else { // for k8s 1.21+, resource.DeviceIds contain only one device ID
						deviceIds = append(deviceIds, resource.DeviceIds...)
					}
				}
			}
			// for k8s 1.21+
			if devices.Equals(types.NewDevice(deviceIds)) {
				klog.Infof("pod %s/%s lodated with device list %v", pod.Namespace, pod.Name, deviceIds)
				return &types.PodContainer{
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Container: container.Name,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("not such pod with the same devices list")
}

func (k *KubeletDeviceLocator) List() ([]*types.PodInfo, error) {
	ans, err := k.client.List(context.Background(), &v1alpha1.ListPodResourcesRequest{})
	if err != nil {
		return nil, err
	}
	// pod -> container -> resource
	list := []*types.PodInfo{}
	for _, pod := range ans.PodResources {
		pi := types.NewPI(pod.Namespace, pod.Name)
		for _, container := range pod.Containers {
			for _, resource := range container.Devices {
				if resource.ResourceName == k.resource {
					pi.ContainerDeviceMap[container.Name] = types.NewDevice(resource.DeviceIds)
				}
			}
		}
		list = append(list, pi)
	}
	return list, err
}

func (k *KubeletDeviceLocator) Close() error {
	return k.conn.Close()
}
