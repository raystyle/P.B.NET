package beacon

import (
	"project/internal/logger"
)

type Beacon struct {
	Debug *Debug
	logLv logger.Level
}

func New(cfg *Config) (*Beacon, error) {
	// init logger
	lv, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := cfg.Debug
	beacon := &Beacon{
		Debug: &debug,
		logLv: lv,
	}
	return beacon, nil
}
