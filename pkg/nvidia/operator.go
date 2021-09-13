package nvidia

type GPUOperator interface {
	DetectNumber() (int, error)
	Create(index int, id string) error
	Delete(index int, id string) error
	Check(index int, id string) bool

	SetPolicy(policy string) error
}

// TODO: We should implement GPUShareOperator here.
// We should make a soft link for the used nvidia device.
// Docker will handle the link and set the device cgroup.
// See: https://github.com/moby/moby/blob/5176095455642c30642efacf6f35afc7c6dede92/oci/devices_linux.go#L41

type GPUShareOperator struct {
	Root string
}

func NewGPUOperator() GPUOperator {
	return &GPUShareOperator{}
}

func (G *GPUShareOperator) DetectNumber() (int, error) {
	panic("implement me")
}

func (G *GPUShareOperator) Create(index int, id string) error {
	panic("implement me")
}

func (G *GPUShareOperator) Delete(index int, id string) error {
	panic("implement me")
}

func (G *GPUShareOperator) Check(index int, id string) bool {
	panic("implement me")
}

func (G *GPUShareOperator) SetPolicy(policy string) error {
	panic("implement me")
}


