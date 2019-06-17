package node

type NODE struct {
	config *Config
	logger *logger
	global *global
}

func New(c *Config) (*NODE, error) {
	node := &NODE{config: c}
	logger, err := new_logger(node)
	if err != nil {
		return nil, err
	}
	node.logger = logger
	global, err := new_global(node)
	if err != nil {
		return nil, err
	}
	node.global = global
	node.config = nil
	return node, nil
}

func (this *NODE) Main() error {
	return nil
}

func (this *NODE) Exit() error {
	return nil
}
