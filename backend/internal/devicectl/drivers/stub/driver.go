// Package stub is an in-memory Driver for tests. It records every Execute call
// and returns configurable results without touching the network.
package stub

import (
	"context"
	"sync"
	"time"

	"smartclass/internal/devicectl"
)

const Name = "stub"

type Call struct {
	Target  devicectl.Target
	Command devicectl.Command
}

type Driver struct {
	mu     sync.Mutex
	calls  []Call
	result devicectl.Result
	err    error
}

func New() *Driver {
	return &Driver{
		result: devicectl.Result{Status: devicectl.StatusUnknown, Online: true, LastSeen: time.Now().UTC()},
	}
}

func (d *Driver) Name() string { return Name }

func (d *Driver) SetResult(r devicectl.Result) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.result = r
}

func (d *Driver) SetError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.err = err
}

func (d *Driver) Calls() []Call {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Call, len(d.calls))
	copy(out, d.calls)
	return out
}

func (d *Driver) Execute(_ context.Context, t devicectl.Target, c devicectl.Command) (devicectl.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, Call{Target: t, Command: c})
	if d.err != nil {
		return devicectl.Result{}, d.err
	}
	res := d.result
	switch c.Type {
	case devicectl.CmdOn:
		res.Status = devicectl.StatusOn
	case devicectl.CmdOff:
		res.Status = devicectl.StatusOff
	case devicectl.CmdOpen:
		res.Status = devicectl.StatusOpen
	case devicectl.CmdClose:
		res.Status = devicectl.StatusClosed
	}
	res.LastSeen = time.Now().UTC()
	return res, nil
}

func (d *Driver) Probe(_ context.Context, _ devicectl.Target) (devicectl.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return devicectl.Result{}, d.err
	}
	return d.result, nil
}
