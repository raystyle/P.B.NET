package node

type Config struct {
}

type NODE struct {
	global *global
}

func New(c *Config) (*NODE, error) {
	return &NODE{}, nil
}

func (this *NODE) Main() error {
	return nil
}

func (this *NODE) Exit() error {
	return nil
}
