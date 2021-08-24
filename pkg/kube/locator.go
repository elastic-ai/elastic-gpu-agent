package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/nano-gpu/nano-gpu-agent/pkg/podresources"
	"github.com/nano-gpu/nano-gpu-agent/pkg/podresources/v1alpha1"
	"github.com/nano-gpu/nano-gpu-agent/pkg/types"
)

type DeviceLocator interface {
	Locate(devices types.DeviceList) (*types.ResourceInfo, error)
	List() ([]*types.ResourceInfo, error)
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

func (k *KubeletDeviceLocator) Locate(devices types.DeviceList) (*types.ResourceInfo, error) {
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
			for _, resource := range container.Devices {
				if resource.ResourceName == k.resource {
					if devices.Equals(resource.DeviceIds) {
						return &types.ResourceInfo{
							Namespace: pod.Namespace,
							Name:      pod.Name,
							Container: container.Name,
							Devices:   devices,
						}, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("not such pod with the same devices list")
}

func (k *KubeletDeviceLocator) List() ([]*types.ResourceInfo, error) {
	ans, err := k.client.List(context.Background(), &v1alpha1.ListPodResourcesRequest{})
	if err != nil {
		return nil, err
	}
	// pod -> container -> resource
	list := []*types.ResourceInfo{}
	for _, pod := range ans.PodResources {
		for _, container := range pod.Containers {
			for _, resource := range container.Devices {
				if resource.ResourceName == k.resource {
					list = append(list, &types.ResourceInfo{
						Namespace: pod.Namespace,
						Name:      pod.Name,
						Container: container.Name,
						Devices: resource.DeviceIds,
					})
				}
			}
		}
	}
	return list, err
}


func (k *KubeletDeviceLocator) Close() error {
	return k.conn.Close()
}
