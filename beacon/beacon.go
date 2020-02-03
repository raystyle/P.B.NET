package beacon

import (
	"github.com/pkg/errors"
)

// Beacon send messages to Controller
type Beacon struct {
	Test *Test

	logger    *gLogger   // global logger
	global    *global    // certificate, proxy, dns, time syncer, and ...
	syncer    *syncer    // sync network guid
	clientMgr *clientMgr // clients manager
	register  *register  // about register to Controller
	worker    *worker    // do work
}

// New is used to create a Beacon from configuration
func New(cfg *Config) (*Beacon, error) {
	// copy test
	test := cfg.Test
	beacon := &Beacon{Test: &test}
	// logger
	lg, err := newLogger(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize logger")
	}
	beacon.logger = lg
	// global
	global, err := newGlobal(beacon.logger, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize global")
	}
	beacon.global = global
	// syncer
	syncer, err := newSyncer(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize syncer")
	}
	beacon.syncer = syncer
	// client manager
	clientMgr, err := newClientManager(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize client manager")
	}
	beacon.clientMgr = clientMgr
	// register
	register, err := newRegister(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize register")
	}
	beacon.register = register

	// worker
	worker, err := newWorker(beacon, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize worker")
	}
	beacon.worker = worker

	return beacon, nil
}
