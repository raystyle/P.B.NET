package beacon

import (
	"project/internal/logger"
)

type BEACON struct {
	Debug *Debug
	logLv logger.Level
}

func New(cfg *Config) (*BEACON, error) {
	// init logger
	lv, err := logger.Parse(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	// copy debug config
	debug := cfg.Debug
	beacon := &BEACON{
		Debug: &debug,
		logLv: lv,
	}
	return beacon, nil
}
