package messages

import (
	"time"

	"project/internal/guid"
	"project/internal/logger"
)

// Bootstrap contains tag, mode and configuration.
type Bootstrap struct {
	Tag    string
	Mode   string
	Config []byte
}

// TestRequest is the test request with id.
type TestRequest struct {
	ID      guid.GUID
	Request []byte
}

// SetID is used to set message id.
func (t *TestRequest) SetID(id *guid.GUID) {
	t.ID = *id
}

// TestResponse is the test response with id.
type TestResponse struct {
	ID       guid.GUID
	Response []byte
}

// PluginRequest is used to wrap plugin.
type PluginRequest struct {
	ID      guid.GUID
	Request []byte // plugin marshal self.
}

// SetID is used to set message id.
func (p *PluginRequest) SetID(id *guid.GUID) {
	p.ID = *id
}

// PluginResponse is used to wrap plugin.
type PluginResponse struct {
	ID       guid.GUID
	Response []byte // plugin unmarshal self.
}

// Log is the Node or Beacon log.
type Log struct {
	Time   time.Time
	Level  logger.Level
	Source string

	// reduce one copy about plain text log
	Log []byte
}
