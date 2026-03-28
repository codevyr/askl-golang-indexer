package var_blank

type Iface interface{ M() }
type Impl struct{}

func (Impl) M() {}

var _ Iface = Impl{}
