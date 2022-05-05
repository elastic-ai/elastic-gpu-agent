package operator

type NvidiaOperator struct {
	baseOperator
}

func NewNvidiaOperator() GPUOperator {
	return &NvidiaOperator{}
}

func (g *NvidiaOperator) Check(index int, id string) bool {
	return true
}

func (g *NvidiaOperator) Create(index int, id string) error {
	return nil
}

func (g *NvidiaOperator) Delete(index int, id string) error {
	return nil
}
