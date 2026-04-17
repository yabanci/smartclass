package device_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/device"
	"smartclass/internal/device/devicetest"
	"smartclass/internal/devicectl"
	"smartclass/internal/devicectl/drivers/stub"
	"smartclass/internal/realtime"
	"smartclass/internal/user"
)

type capturingBroker struct {
	mu     sync.Mutex
	events []realtime.Event
}

func (b *capturingBroker) Publish(_ context.Context, e realtime.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
	return nil
}
func (b *capturingBroker) Events() []realtime.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]realtime.Event(nil), b.events...)
}

type fixture struct {
	svc       *device.Service
	clsSvc    *classroom.Service
	repo      *devicetest.MemRepo
	factory   *devicectl.Factory
	driver    *stub.Driver
	broker    *capturingBroker
	classroom *classroom.Classroom
	owner     classroom.Principal
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clsRepo := classroomtest.NewMemRepo()
	clsSvc := classroom.NewService(clsRepo)
	ctx := context.Background()

	ownerID := uuid.New()
	owner := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	cls, err := clsSvc.Create(ctx, classroom.CreateInput{Name: "C", CreatedBy: ownerID})
	require.NoError(t, err)

	devRepo := devicetest.NewMemRepo()
	factory := devicectl.NewFactory()
	driver := stub.New()
	factory.Register(driver)
	broker := &capturingBroker{}

	svc := device.NewService(devRepo, clsSvc, factory, broker)

	return &fixture{
		svc: svc, clsSvc: clsSvc, repo: devRepo, factory: factory,
		driver: driver, broker: broker, classroom: cls, owner: owner,
	}
}

func TestService_Create(t *testing.T) {
	f := newFixture(t)

	t.Run("ok", func(t *testing.T) {
		d, err := f.svc.Create(context.Background(), f.owner, device.CreateInput{
			ClassroomID: f.classroom.ID, Name: "Light 1", Type: "light",
			Brand: "generic", Driver: stub.Name, Config: map[string]any{"baseUrl": "http://x"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Light 1", d.Name)
		assert.NotEqual(t, uuid.Nil, d.ID)

		events := f.broker.Events()
		require.NotEmpty(t, events)
		assert.Equal(t, "device.created", events[len(events)-1].Type)
	})

	t.Run("unknown driver rejected", func(t *testing.T) {
		_, err := f.svc.Create(context.Background(), f.owner, device.CreateInput{
			ClassroomID: f.classroom.ID, Name: "X", Type: "light",
			Brand: "generic", Driver: "ghost",
		})
		require.ErrorIs(t, err, device.ErrUnknownDriver)
	})

	t.Run("outsider rejected", func(t *testing.T) {
		outsider := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
		_, err := f.svc.Create(context.Background(), outsider, device.CreateInput{
			ClassroomID: f.classroom.ID, Name: "X", Type: "light",
			Brand: "generic", Driver: stub.Name,
		})
		require.ErrorIs(t, err, classroom.ErrForbidden)
	})
}

func TestService_Execute(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	d, err := f.svc.Create(ctx, f.owner, device.CreateInput{
		ClassroomID: f.classroom.ID, Name: "L", Type: "light",
		Brand: "generic", Driver: stub.Name,
	})
	require.NoError(t, err)

	t.Run("ON updates status and publishes event", func(t *testing.T) {
		res, err := f.svc.Execute(ctx, f.owner, d.ID, devicectl.Command{Type: devicectl.CmdOn})
		require.NoError(t, err)
		assert.Equal(t, string(devicectl.StatusOn), res.Status)
		assert.True(t, res.Online)

		calls := f.driver.Calls()
		require.Len(t, calls, 1)
		assert.Equal(t, devicectl.CmdOn, calls[0].Command.Type)

		events := f.broker.Events()
		require.NotEmpty(t, events)
		assert.Equal(t, "device.state_changed", events[len(events)-1].Type)
	})

	t.Run("unsupported command rejected before driver", func(t *testing.T) {
		_, err := f.svc.Execute(ctx, f.owner, d.ID, devicectl.Command{Type: "NUKE"})
		require.ErrorIs(t, err, device.ErrUnsupportedCmd)
	})

	t.Run("driver error marks offline", func(t *testing.T) {
		f.driver.SetError(devicectl.ErrUnavailable)
		defer f.driver.SetError(nil)

		_, err := f.svc.Execute(ctx, f.owner, d.ID, devicectl.Command{Type: devicectl.CmdOff})
		require.Error(t, err)

		got, err := f.repo.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.False(t, got.Online)
	})
}

func TestService_ListByClassroom(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.svc.Create(ctx, f.owner, device.CreateInput{
		ClassroomID: f.classroom.ID, Name: "A", Type: "light", Brand: "g", Driver: stub.Name,
	})
	require.NoError(t, err)
	_, err = f.svc.Create(ctx, f.owner, device.CreateInput{
		ClassroomID: f.classroom.ID, Name: "B", Type: "relay", Brand: "g", Driver: stub.Name,
	})
	require.NoError(t, err)

	list, err := f.svc.ListByClassroom(ctx, f.owner, f.classroom.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
