package kube

import (
	"context"
	"elasticgpu.io/elastic-gpu-agent/pkg/common"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type Sitter interface {
	Start()
	GetPod(namespace, name string) (*v1.Pod, error)
	GetPodFromApiServer(namespace, name string) (*v1.Pod, error)
	GetNodeFromApiServer(name string) (*v1.Node, error)
	HasSynced() bool
}

type PodSitter struct {
	client           *kubernetes.Clientset
	informersFactory informers.SharedInformerFactory
	podLister        listerv1.PodLister
	podInformer      cache.SharedIndexInformer
}

func (p *PodSitter) Start() {
	p.informersFactory.Start(common.NeverStop)
}

func (p *PodSitter) GetPod(namespace, name string) (*v1.Pod, error) {
	return p.podLister.Pods(namespace).Get(name)
}

func (p *PodSitter) GetPodFromApiServer(namespace, name string) (*v1.Pod, error) {
	return p.client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (p *PodSitter) GetNodeFromApiServer(name string) (*v1.Node, error) {
	return p.client.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
}

func (p *PodSitter) HasSynced() bool {
	if p.podInformer == nil {
		return false
	}

	return p.podInformer.HasSynced()
}

func NewSitter(client *kubernetes.Clientset, nodeName string, deleteHook func(interface{})) Sitter {
	ps := &PodSitter{
		client:           client,
		informersFactory: informers.NewSharedInformerFactoryWithOptions(client, time.Second, informers.WithTweakListOptions(nodeNameFilter(nodeName))),
	}
	ps.podInformer = ps.informersFactory.Core().V1().Pods().Informer()
	ps.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			deleteHook(obj)
		},
	})
	ps.podLister = ps.informersFactory.Core().V1().Pods().Lister()
	return ps
}

func nodeNameFilter(nodeName string) func(options *metav1.ListOptions) {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector(common.NodeNameField, string(nodeName)).String()
	}
}
