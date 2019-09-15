package node

type sender struct {
}

func newSender(ctx *NODE, cfg *Config) (*sender, error) {
	sender := sender{}
	return &sender, nil
}
