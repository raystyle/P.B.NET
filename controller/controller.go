package controller

type CONTROLLER struct {
	config *Config
}

func New(c *Config) (*CONTROLLER, error) {
	controller := &CONTROLLER{config: c}

	return controller, nil
}

func (this *CONTROLLER) Main() error {

	return nil
}

func (this *CONTROLLER) Exit() error {

	return nil
}
