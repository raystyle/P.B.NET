package control

import (
	"context"
	"sync/atomic"
)

// states about controller
const (
	_ int32 = iota
	StateRunning
	StatePaused
	StateCancel
)

// Controller is is used to pause current loop.
type Controller struct {
	ctx context.Context

	state   *int32
	pauseCh chan struct{}
}

// NewController is used to create a controller.
func NewController(ctx context.Context) *Controller {
	state := StateRunning
	return &Controller{
		ctx:     ctx,
		state:   &state,
		pauseCh: make(chan struct{}, 1),
	}
}

// Pause is used to pause current loop.
func (ctrl *Controller) Pause() {
	atomic.StoreInt32(ctrl.state, StatePaused)
}

// Continue is used to continue current loop.
func (ctrl *Controller) Continue() {
	if atomic.LoadInt32(ctrl.state) != StatePaused {
		return
	}
	select {
	case ctrl.pauseCh <- struct{}{}:
	default:
	}
}

// Paused is used to check need pause current loop.
func (ctrl *Controller) Paused() {
	if atomic.LoadInt32(ctrl.state) != StatePaused {
		return
	}
	select {
	case <-ctrl.pauseCh:
		atomic.StoreInt32(ctrl.state, StateRunning)
	case <-ctrl.ctx.Done():
		atomic.StoreInt32(ctrl.state, StateCancel)
	}
}

// State is used to get current state.
func (ctrl *Controller) State() int32 {
	return atomic.LoadInt32(ctrl.state)
}
