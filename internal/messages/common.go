package messages

import (
	"time"

	"project/internal/logger"
)

// TestRequest is the test request with id.
type TestRequest struct {
	ID      uint64
	Request []byte
}

// SetID is used to set message id.
func (t *TestRequest) SetID(id uint64) {
	t.ID = id
}

// TestResponse is the test response with id.
type TestResponse struct {
	ID       uint64
	Response []byte
}

// PluginRequest is used to wrap plugin.
type PluginRequest struct {
	ID      uint64
	Request []byte // plugin marshal self.
}

// SetID is used to set message id.
func (p *PluginRequest) SetID(id uint64) {
	p.ID = id
}

// PluginResponse is used to wrap plugin.
type PluginResponse struct {
	ID       uint64
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
