// Package devicectl defines a hardware-agnostic Driver interface for controlling
// smart devices. Concrete drivers (Tuya/Shelly/Sonoff/Aqara/HA/PJLink/MQTT) each
// live under drivers/<name> and self-register with a Factory.
package devicectl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type CommandType string

const (
	CmdOn       CommandType = "ON"
	CmdOff      CommandType = "OFF"
	CmdOpen     CommandType = "OPEN"
	CmdClose    CommandType = "CLOSE"
	CmdSetValue CommandType = "SET_VALUE"
)

func (c CommandType) Valid() bool {
	switch c {
	case CmdOn, CmdOff, CmdOpen, CmdClose, CmdSetValue:
		return true
	}
	return false
}

type Command struct {
	Type  CommandType
	Value any
}

type Target struct {
	ID     uuid.UUID
	Brand  string
	Config map[string]any
}

type Status string

const (
	StatusOn      Status = "on"
	StatusOff     Status = "off"
	StatusOpen    Status = "open"
	StatusClosed  Status = "closed"
	StatusUnknown Status = "unknown"
)

type Result struct {
	Status    Status
	Online    bool
	LastSeen  time.Time
	Raw       map[string]any
}

var (
	ErrUnsupportedCommand = errors.New("devicectl: unsupported command")
	ErrInvalidConfig      = errors.New("devicectl: invalid config")
	ErrDriverNotFound     = errors.New("devicectl: driver not found")
	ErrUnavailable        = errors.New("devicectl: device unavailable")
)

// Driver is a brand/protocol-specific adapter. Implementations must be safe for
// concurrent use.
type Driver interface {
	Name() string
	Execute(ctx context.Context, target Target, cmd Command) (Result, error)
	Probe(ctx context.Context, target Target) (Result, error)
}
