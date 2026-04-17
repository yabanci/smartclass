package scene_test

import (
	"context"
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
	"smartclass/internal/scene"
	"smartclass/internal/scene/scenetest"
	"smartclass/internal/user"
)

type fixture struct {
	svc     *scene.Service
	deviceA uuid.UUID
	deviceB uuid.UUID
	owner   classroom.Principal
	classID uuid.UUID
	driver  *stub.Driver
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	clsRepo := classroomtest.NewMemRepo()
	clsSvc := classroom.NewService(clsRepo)
	ownerID := uuid.New()
	owner := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	cls, err := clsSvc.Create(ctx, classroom.CreateInput{Name: "R", CreatedBy: ownerID})
	require.NoError(t, err)

	devRepo := devicetest.NewMemRepo()
	factory := devicectl.NewFactory()
	driver := stub.New()
	factory.Register(driver)
	devSvc := device.NewService(devRepo, clsSvc, factory, realtime.Noop{})

	a, err := devSvc.Create(ctx, owner, device.CreateInput{
		ClassroomID: cls.ID, Name: "A", Type: "light", Brand: "g", Driver: stub.Name,
	})
	require.NoError(t, err)
	b, err := devSvc.Create(ctx, owner, device.CreateInput{
		ClassroomID: cls.ID, Name: "B", Type: "relay", Brand: "g", Driver: stub.Name,
	})
	require.NoError(t, err)

	sceneSvc := scene.NewService(scenetest.NewMemRepo(), clsSvc, devSvc, realtime.Noop{})

	return &fixture{svc: sceneSvc, deviceA: a.ID, deviceB: b.ID, owner: owner, classID: cls.ID, driver: driver}
}

func TestService_RunsAllSteps(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sc, err := f.svc.Create(ctx, f.owner, scene.CreateInput{
		ClassroomID: f.classID, Name: "Lesson", Steps: []scene.Step{
			{DeviceID: f.deviceA, Command: "ON"},
			{DeviceID: f.deviceB, Command: "OFF"},
		},
	})
	require.NoError(t, err)

	res, err := f.svc.Run(ctx, f.owner, sc.ID)
	require.NoError(t, err)
	require.Len(t, res.Steps, 2)
	assert.True(t, res.Steps[0].Success)
	assert.True(t, res.Steps[1].Success)

	calls := f.driver.Calls()
	assert.Len(t, calls, 2)
}

func TestService_PartialFailure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sc, err := f.svc.Create(ctx, f.owner, scene.CreateInput{
		ClassroomID: f.classID, Name: "Bad", Steps: []scene.Step{
			{DeviceID: f.deviceA, Command: "ON"},
			{DeviceID: uuid.New(), Command: "ON"}, // device doesn't exist
		},
	})
	require.NoError(t, err)

	res, err := f.svc.Run(ctx, f.owner, sc.ID)
	require.Error(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Steps[0].Success)
	assert.False(t, res.Steps[1].Success)
}

func TestService_CRUD(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sc, err := f.svc.Create(ctx, f.owner, scene.CreateInput{ClassroomID: f.classID, Name: "X"})
	require.NoError(t, err)

	newName := "Y"
	updated, err := f.svc.Update(ctx, f.owner, sc.ID, scene.UpdateInput{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "Y", updated.Name)

	list, err := f.svc.ListByClassroom(ctx, f.owner, f.classID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, f.svc.Delete(ctx, f.owner, sc.ID))
	_, err = f.svc.Get(ctx, f.owner, sc.ID)
	assert.ErrorIs(t, err, scene.ErrDomainNotFound)
}
