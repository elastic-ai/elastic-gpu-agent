package kube

import (
	"context"
	"manager/pkg/common"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	v1 "k8s.io/api/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
)

type Sitter interface {
	Start()
	GetPod(namespace, name string) (*v1.Pod, error)
	GetPodFromApiServer(namespace, name string) (*v1.Pod, error)
}

type PodSitter struct {
	client           *kubernetes.Clientset
	informersFactory informers.SharedInformerFactory
	podLister        listerv1.PodLister
	podInformer      cache.SharedIndexInformer
	once             sync.Once
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

func NewSitter(client *kubernetes.Clientset, nodeName string, deleteHook func()) Sitter {
	ps := &PodSitter{
		client:           client,
		informersFactory: informers.NewSharedInformerFactoryWithOptions(client,
			time.Second,
			informers.WithTweakListOptions(nodeNameFilter(nodeName)),
			informers.WithTweakListOptions(labelFilter(common.NanoGPUAssumedLabel))),
	}
	ps.podInformer = ps.informersFactory.Core().V1().Pods().Informer()
	ps.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if _, ok := pod.Annotations[common.NanoGPUAssumedAnnotation]; ok {
				deleteHook()
			}
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

func labelFilter(label string) func(options *metav1.ListOptions) {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = label
	}
}