package messages

import (
	"time"

	"project/internal/guid"
)

// ChangeMode is used to change interactive mode.
// Controller will send it.
type ChangeMode struct {
	ID          guid.GUID
	Interactive bool
}

// SetID is used to set message id.
func (cm *ChangeMode) SetID(id *guid.GUID) {
	cm.ID = *id
}

// ChangeModeResult is used to notice Beacon mode has
// been changed successfully, if failed to change mode,
// Err will include the reason.
type ChangeModeResult struct {
	ID  guid.GUID
	Err string
}

// InteractiveIdleTimeout is used to disable interactive mode
// actively, Beacon will check deadline.
const InteractiveIdleTimeout = time.Minute

// reason about Beacon change mode.
const (
	// controller change mode actively.
	ModeChangeReasonCtrlCommand = iota + 1

	// controller don't do anything more than 5 minute.
	ModeChangedReasonNoNewCommand

	// Beacon can't query interactive status.
	ModeChangedReasonFailedToQuery

	// Beacon query mode and return disable interactive mode.
	ModeChangedReasonDisabled
)

// ModeChanged is used to notice Controller that Beacon's
// mode has been changed. Beacon will send it.
type ModeChanged struct {
	Interactive bool
	Reason      string
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
