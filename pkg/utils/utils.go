package utils

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func NewClientInCluster() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func MustNewClientInCluster() (*kubernetes.Clientset) {
	client, err := NewClientInCluster()
	if err != nil {
		panic(err)
	}
	return client
}

func ExitSignal() <-chan os.Signal {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT)
	return ch
}

func DumpSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGUSR1)
	for range ch {
		if _, err := DumpStacks("/var/log"); err != nil {
			klog.Error(err.Error())
		}
	}
}

func DumpStacks(dir string) (string, error) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	var f *os.File
	if dir != "" {
		path := filepath.Join(dir, fmt.Sprintf("goroutine-stacks-%s.log", strings.Replace(time.Now().Format(time.RFC3339), ":", "", -1)))
		var err error
		f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return "", fmt.Errorf("failed to open file to write the goroutine stacks: %s", err.Error())
		}
		defer f.Close()
		defer f.Sync()
	} else {
		f = os.Stderr
	}
	if _, err := f.Write(buf); err != nil {
		return "", fmt.Errorf("failed to write goroutine stacks: %s", err.Error())
	}
	return f.Name(), nil
}
