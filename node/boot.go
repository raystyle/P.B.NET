package node

type boot struct {
}

func newBoot() (*boot, error) {

	return new(boot), nil
}
