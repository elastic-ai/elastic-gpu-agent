package storage

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"

	"github.com/nano-gpu/nano-gpu-agent/pkg/types"

	"k8s.io/apimachinery/pkg/util/rand"
)

var (
	storage Storage
	tmpfile string
)

func TestMain(m *testing.M) {
	var err     error
	tmpfile = path.Join("/tmp", rand.String(8))
	defer os.RemoveAll(tmpfile)
	storage, err = NewStorage(tmpfile)
	if err != nil {
		fmt.Printf("create boltdb failed: %s", err.Error())
		return
	}
	m.Run()
}

func TestSaveAndLoad(t *testing.T) {
	var err error
	pods := []types.PodInfo{{
		Namespace: "default",
		Name:      "pod",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod1",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod2",
		ContainerDevices: nil,
	}}
	for _, pod := range pods {
		err := storage.Save(&pod)
		assert.Nil(t, err)
	}
	assert.Nil(t, storage.Close())
	storage, err = NewStorage(tmpfile)
	assert.Nil(t, err)
	for _, pod := range pods {
		loadPod, err := storage.Load(pod.Namespace, pod.Name)
		assert.Nil(t, err)
		assert.Equal(t, &pod, loadPod)
	}
}

func TestLoadNil(t *testing.T) {
	pods := []types.PodInfo{{
		Namespace: "default",
		Name:      "pod",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod1",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod2",
		ContainerDevices: nil,
	}}
	for _, pod := range pods {
		err := storage.Save(&pod)
		assert.Nil(t, err)
	}
	_, err := storage.Load("kube-system", "kube-proxy")
	assert.NotNil(t, err)

	pod := storage.LoadOrCreate("kube-system", "kube-proxy")
	assert.NotNil(t, pod)
	assert.Equal(t, pod.Name, "kube-proxy")
	assert.Equal(t, pod.Namespace, "kube-system")
}

func TestDelete(t *testing.T) {
	pods := []types.PodInfo{{
		Namespace: "default",
		Name:      "pod",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod1",
		ContainerDevices: map[string]types.Device{"container": {
			PGPU:   0,
			Core:   100,
			Memory: 100,
			List:   []string{"a", "b", "c"},
		}},
	}, {
		Namespace: "default",
		Name:      "pod2",
		ContainerDevices: nil,
	}}
	for _, pod := range pods {
		err := storage.Save(&pod)
		assert.Nil(t, err)
	}
	for _, pod := range pods {
		err := storage.Delete(pod.Namespace, pod.Name)
		assert.Nil(t, err)
	}
	for _, pod := range pods {
		_, err := storage.Load(pod.Namespace, pod.Name)
		assert.NotNil(t, err)
	}
}