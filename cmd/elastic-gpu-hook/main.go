package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	logFileName = "/var/log/nvidia-prestart-hook.log"
)

func setLog() {
	logfile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logfile)
	log.SetPrefix(time.Now().Format("2006-01-02 15:04:05") + "[" + fmt.Sprintf("%d", time.Now().UnixNano()) + "]" + " [Prestart] ")
	log.SetFlags(log.Lshortfile)
}

func loadSpec(path string) (spec spec.Spec, err error) {
	f, err := os.Open(path)
	if err != nil {
		log.Panicln("could not open OCI spec:", err)
		return
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		log.Panicln("could not decode OCI spec:", err)
		return
	}
	if spec.Version == "" {
		err = fmt.Errorf("Version is empty in OCI spec")
		return
	}
	if spec.Process == nil {
		err = fmt.Errorf("Process is empty in OCI spec")
		return
	}
	if spec.Root == nil {
		err = fmt.Errorf("Root is empty in OCI spec")
		return
	}

	return
}

func getEnvFromSpec(envName string, envs []string) string {
	envName = envName + "="

	for _, env := range envs {
		if strings.HasPrefix(env, envName) {
			idx := strings.Index(env, "=")
			if idx != -1 {
				return env[idx+1:]
			}
		}
	}

	return ""
}

func getGPUIndex(file string) (int, error) {
	nvidiaPath, err := os.Readlink(file)
	if err != nil {
		log.Fatal(err)
	}
	nvidiaIndex := strings.Split(nvidiaPath, "/")[2]
	idx := nvidiaIndex[6:]
	return strconv.Atoi(idx)
}

func findGPUIndexes(gpu string) ([]int, error) {
	devDir := "/dev"
	devFiles, err := os.ReadDir(devDir)
	if err != nil {
		return nil, err
	}
	isGpuSymlink := func(f fs.DirEntry) bool {
		if f.Type()&fs.ModeSymlink != 0 && strings.HasPrefix(f.Name(), fmt.Sprintf("elastic-gpu-%s", gpu)) {
			return true
		}
		return false
	}

	gpuIndexes := make([]int, 0)
	for _, f := range devFiles {
		if isGpuSymlink(f) {
			gpuIndex, err := getGPUIndex(fmt.Sprintf("%s/%s", devDir, f.Name()))
			if err != nil {
				fmt.Printf("failed to parse gpu index from %s to integer: %v\n", f.Name(), err)
				continue
			}
			gpuIndexes = append(gpuIndexes, gpuIndex)
		}
	}

	return gpuIndexes, nil
}

func main() {
	setLog()

	log.Printf("Copy stdin to prestart hook\n")
	hookSpecBuf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Printf("Fail to read from stdin")
		return
	}
	log.Printf("hookSpecBuf: %s\n", hookSpecBuf)

	containerSpec := spec.Spec{}

	hookSpec := make(map[string]interface{})
	decoder := json.NewDecoder(strings.NewReader(string(hookSpecBuf)))
	decoder.UseNumber()
	if err := decoder.Decode(&hookSpec); err != nil {
		log.Printf("Fail to decode hook spec: %v", err)
		return
	}
	log.Printf("data: %#+v\n", hookSpec)

	bundleElem, exists := hookSpec["bundle"]
	if !exists {
		log.Printf("Did not find bundle in hookSpec\n")
		return
	}
	bundle, ok := bundleElem.(string)
	if !ok {
		log.Printf("Bundle is not a string")
		return
	}
	log.Printf("Get bundle: %s", string(bundle))

	specFile := path.Join(bundle, "config.json")
	log.Printf("Container spec file path:%s\n", specFile)

	containerSpec, err = loadSpec(specFile)
	if err != nil {
		log.Printf("Fail to get container spec %v\n", err)
		return
	}

	gpu := getEnvFromSpec("GPU", containerSpec.Process.Env)
	log.Println("containerSpec.Process.Env:", containerSpec.Process.Env)
	if gpu == "" {
		log.Print("No elastic GPU specified. Do prestart as non elastic-gpu.")
		err := doPreStart(nil, hookSpecBuf)
		if err != nil {
			log.Printf("failed to do prestart: %v\n", err)
		}
		return
	}

	gpuIndexes, err := findGPUIndexes(gpu)
	if err != nil {
		log.Printf("find gpu index failed: %s", err.Error())
		return
	}
	log.Printf("gpu ids: %+v", gpuIndexes)

	if err := doPreStart(gpuIndexes, hookSpecBuf); err != nil {
		log.Printf("failed to do prestart: %v\n", err)
		return
	}
}

func doPreStart(gpuIndexes []int, hookSpecBuf []byte) error {
	var prestart *exec.Cmd

	if len(gpuIndexes) > 0 {
		ids := make([]string, 0)
		for _, id := range gpuIndexes {
			ids = append(ids, strconv.Itoa(id))
		}
		prestart = exec.Command("/usr/bin/nvidia-container-toolkit", "prestart", strings.Join(ids, ","))
	} else {
		prestart = exec.Command("/usr/bin/nvidia-container-toolkit", "prestart")
	}
	prestartStdin, err := prestart.StdinPipe()
	if err != nil {
		return fmt.Errorf("Fail to get stdin pipe: %v", err)
	}
	go func() {
		defer prestartStdin.Close()
		// Should block until stdin pipe ready
		if _, err := prestartStdin.Write(hookSpecBuf); err != nil {
			log.Printf("Write to toolkit failed: %v\n", err)
			return
		}
	}()

	output, err := prestart.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Prestart exec failed:%v", err)
	}
	log.Printf("Prestart output: %s", string(output))

	return nil

}
