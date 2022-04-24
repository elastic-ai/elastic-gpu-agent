module elasticgpu.io/elastic-gpu-agent

go 1.16

require (
	elasticgpu.io/elastic-gpu v0.0.0
	github.com/NVIDIA/go-nvml v0.11.2-1
	github.com/boltdb/bolt v1.3.1
	github.com/fsnotify/fsnotify v1.5.1
	github.com/gogo/protobuf v1.3.2
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog v1.0.0
	k8s.io/kubelet v0.20.2
)

replace elasticgpu.io/elastic-gpu => ../elastic-gpu
