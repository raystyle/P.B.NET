package messages

import (
	"time"
)

// ChangeMode is used to change interactive mode.
// Controller will send it.
type ChangeMode struct {
	ID          uint64
	Interactive bool
}

// SetID is used to set message id.
func (cm *ChangeMode) SetID(id uint64) {
	cm.ID = id
}

// QueryModeStatus is used to check Controller is
// still set the Beacon in interactive mode.
// Node will send it.
type QueryModeStatus struct {
	Time time.Time
}

// AnswerModeStatus is used to answer Beacon the
// status about itself.
// Controller will send it.
type AnswerModeStatus struct {
	Interactive bool
}
