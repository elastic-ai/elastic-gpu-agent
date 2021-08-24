package kube

import (
	"github.com/nano-gpu/nano-gpu-agent/pkg/config"
	"github.com/nano-gpu/nano-gpu-scheduler/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"time"
)

type Sitter interface {
	Start()
	GetPod(namespace, name string) (*v1.Pod, error)
}

type PodSitter struct {
	client           *kubernetes.Clientset
	informersFactory informers.SharedInformerFactory
	podLister        listerv1.PodLister
	podInformer      cache.SharedIndexInformer
}

func (p *PodSitter) Start() {
	p.informersFactory.Start(config.NeverStop)
}

func (p *PodSitter) GetPod(namespace, name string) (*v1.Pod, error) {
	return p.podLister.Pods(namespace).Get(name)
}

func NewSitter(client *kubernetes.Clientset, nodeName string, deleteHook func()) Sitter {
	ps := &PodSitter{
		client:           client,
		informersFactory: informers.NewSharedInformerFactoryWithOptions(client, time.Second, informers.WithTweakListOptions(nodeNameFilter(nodeName))),
	}
	ps.podInformer = ps.informersFactory.Core().V1().Pods().Informer()
	ps.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if _, ok := pod.Annotations[types.AnnotationGPUAssume]; ok {
				deleteHook()
			}
		},
	})
	ps.podLister = ps.informersFactory.Core().V1().Pods().Lister()
	return ps
}

func nodeNameFilter(nodeName string) func(options *metav1.ListOptions) {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector(config.NodeNameField, string(nodeName)).String()
	}
}
