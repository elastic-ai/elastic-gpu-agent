package nvidia

type GPUOperator interface {
	DetectNumber() (int, error)
	Create(index int, id string) error
	Delete(index int, id string) error
	Check(index int, id string) bool

	SetPolicy(policy string) error
}

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


