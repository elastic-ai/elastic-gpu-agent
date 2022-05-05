# Elastic GPU Agent
*Elastic GPU Agent* is a Kubernetes device plugin implement for gpu allocation and use in container. It runs as a Daemonset in Kubernetes node. It works as follows:

- Register gpu core and memory resources on node
- Allocate and share gpu resources for containers
- Support gpu resources qos and isolation with specific gpu driver(e.g. elastic gpu)

For the complete solution and further details, please refer to Elastic GPU Scheduler.

## Prerequisites
- Kubernetes v1.17+
- golang 1.16+
- [NVIDIA drivers](https://github.com/NVIDIA/nvidia-docker/wiki/Frequently-Asked-Questions#how-do-i-install-the-nvidia-driver) 
- [nvidia-docker](https://github.com/NVIDIA/nvidia-docker) 
- set `nvidia` as docker `default-runtime`:  add `"default-runtime": "nvidia"` to `/etc/docker/daemon.json`, and restart docker daemon.
  
## Build Image

Run `make` or `TAG=<image-tag> make` to build elastic-gpu-agent image
## Getting Started
Deploy Elastic GPU Agent as follows:

```
$ kubectl apply -f deploy/elastic-gpu-agent.yaml
```
You can find more details on [Elastic GPU Scheduler](https://github.com/elastic-ai/elastic-gpu-scheduler).

## License

Distributed under the [Apache License](./LICENSE).
