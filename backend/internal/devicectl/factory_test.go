package devicectl_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/devicectl"
)

type fakeDriver struct{ name string }

func (f *fakeDriver) Name() string { return f.name }
func (f *fakeDriver) Execute(_ context.Context, _ devicectl.Target, _ devicectl.Command) (devicectl.Result, error) {
	return devicectl.Result{Status: devicectl.StatusOn, Online: true}, nil
}
func (f *fakeDriver) Probe(_ context.Context, _ devicectl.Target) (devicectl.Result, error) {
	return devicectl.Result{Status: devicectl.StatusUnknown}, nil
}

func TestFactory_RegisterAndGet(t *testing.T) {
	f := devicectl.NewFactory()
	f.Register(&fakeDriver{name: "shelly"})
	f.Register(&fakeDriver{name: "tuya"})

	d, err := f.Get("shelly")
	require.NoError(t, err)
	assert.Equal(t, "shelly", d.Name())

	_, err = f.Get("ghost")
	assert.ErrorIs(t, err, devicectl.ErrDriverNotFound)

	assert.ElementsMatch(t, []string{"shelly", "tuya"}, f.Names())
}

func TestFactory_RegisterNilNoop(t *testing.T) {
	f := devicectl.NewFactory()
	f.Register(nil)
	assert.Empty(t, f.Names())
}

func TestCommandType_Valid(t *testing.T) {
	for _, ok := range []devicectl.CommandType{devicectl.CmdOn, devicectl.CmdOff, devicectl.CmdOpen, devicectl.CmdClose, devicectl.CmdSetValue} {
		assert.True(t, ok.Valid(), string(ok))
	}
	assert.False(t, devicectl.CommandType("EXPLODE").Valid())
}
