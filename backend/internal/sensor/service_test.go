package sensor_test

import (
	"context"
	"testing"
	"time"

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
	"smartclass/internal/sensor"
	"smartclass/internal/sensor/sensortest"
	"smartclass/internal/user"
)

type fixture struct {
	svc     *sensor.Service
	repo    *sensortest.MemRepo
	owner   classroom.Principal
	classID uuid.UUID
	devID   uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	clsSvc := classroom.NewService(classroomtest.NewMemRepo())
	ownerID := uuid.New()
	owner := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	cls, _ := clsSvc.Create(ctx, classroom.CreateInput{Name: "R", CreatedBy: ownerID})

	factory := devicectl.NewFactory()
	factory.Register(stub.New())
	devSvc := device.NewService(devicetest.NewMemRepo(), clsSvc, factory, realtime.Noop{})
	dev, _ := devSvc.Create(ctx, owner, device.CreateInput{
		ClassroomID: cls.ID, Name: "T", Type: "sensor", Brand: "aqara", Driver: stub.Name,
	})

	repo := sensortest.NewMemRepo()
	repo.SetDeviceClassroom(dev.ID, cls.ID)
	svc := sensor.NewService(repo, clsSvc, devSvc, realtime.Noop{})

	return &fixture{svc: svc, repo: repo, owner: owner, classID: cls.ID, devID: dev.ID}
}

func TestService_Ingest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		n, err := f.svc.Ingest(ctx, f.owner, []sensor.IngestItem{
			{DeviceID: f.devID, Metric: sensor.MetricTemperature, Value: 22.5, Unit: "C"},
			{DeviceID: f.devID, Metric: sensor.MetricHumidity, Value: 40, Unit: "%"},
		})
		require.NoError(t, err)
		assert.Equal(t, 2, n)
	})

	t.Run("invalid metric", func(t *testing.T) {
		_, err := f.svc.Ingest(ctx, f.owner, []sensor.IngestItem{
			{DeviceID: f.devID, Metric: "weight", Value: 1},
		})
		require.ErrorIs(t, err, sensor.ErrInvalidMetric)
	})

	t.Run("unknown device rejected", func(t *testing.T) {
		_, err := f.svc.Ingest(ctx, f.owner, []sensor.IngestItem{
			{DeviceID: uuid.New(), Metric: sensor.MetricTemperature, Value: 1},
		})
		require.Error(t, err)
	})
}

// TestIngest_BatchWithUnauthorizedDevice verifies that a batch containing a
// device from a classroom the caller doesn't belong to is rejected entirely.
// This tests the all-or-nothing semantics: the authorized device's readings
// must also not be persisted when the batch fails.
func TestIngest_BatchWithUnauthorizedDevice(t *testing.T) {
	ctx := context.Background()

	// Build shared classroom + device services so both classrooms are visible to devSvc.
	clsRepo := classroomtest.NewMemRepo()
	clsSvc := classroom.NewService(clsRepo)

	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	owner := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	otherOwner := classroom.Principal{UserID: otherOwnerID, Role: user.RoleTeacher}

	ownCls, _ := clsSvc.Create(ctx, classroom.CreateInput{Name: "Mine", CreatedBy: ownerID})
	otherCls, _ := clsSvc.Create(ctx, classroom.CreateInput{Name: "Theirs", CreatedBy: otherOwnerID})

	factory := devicectl.NewFactory()
	factory.Register(stub.New())
	devRepo := devicetest.NewMemRepo()
	devSvc := device.NewService(devRepo, clsSvc, factory, realtime.Noop{})

	ownDev, _ := devSvc.Create(ctx, owner, device.CreateInput{
		ClassroomID: ownCls.ID, Name: "Mine", Type: "sensor", Brand: "x", Driver: stub.Name,
	})
	otherDev, _ := devSvc.Create(ctx, otherOwner, device.CreateInput{
		ClassroomID: otherCls.ID, Name: "Theirs", Type: "sensor", Brand: "x", Driver: stub.Name,
	})

	sensorRepo := sensortest.NewMemRepo()
	sensorRepo.SetDeviceClassroom(ownDev.ID, ownCls.ID)
	svc := sensor.NewService(sensorRepo, clsSvc, devSvc, realtime.Noop{})

	// Batch: one authorized device (own) + one unauthorized (other classroom).
	_, err := svc.Ingest(ctx, owner, []sensor.IngestItem{
		{DeviceID: ownDev.ID, Metric: sensor.MetricTemperature, Value: 22},
		{DeviceID: otherDev.ID, Metric: sensor.MetricTemperature, Value: 25},
	})
	require.Error(t, err, "batch with unauthorized device must be rejected")

	// The first (authorized) device's reading must not have been persisted.
	list, err := svc.History(ctx, owner, ownDev.ID, sensor.MetricTemperature, nil, nil, 0)
	require.NoError(t, err)
	assert.Empty(t, list, "no readings should be saved when batch is rejected")
}

func TestService_History(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		_, err := f.svc.Ingest(ctx, f.owner, []sensor.IngestItem{
			{DeviceID: f.devID, Metric: sensor.MetricTemperature, Value: float64(20 + i), RecordedAt: &at},
		})
		require.NoError(t, err)
	}

	list, err := f.svc.History(ctx, f.owner, f.devID, sensor.MetricTemperature, nil, nil, 0)
	require.NoError(t, err)
	assert.Len(t, list, 3)
	assert.InDelta(t, 22, list[0].Value, 0.001)
}

func TestService_Latest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.Ingest(ctx, f.owner, []sensor.IngestItem{
		{DeviceID: f.devID, Metric: sensor.MetricTemperature, Value: 22},
		{DeviceID: f.devID, Metric: sensor.MetricTemperature, Value: 23},
		{DeviceID: f.devID, Metric: sensor.MetricHumidity, Value: 40},
	})
	require.NoError(t, err)

	list, err := f.svc.LatestForClassroom(ctx, f.owner, f.classID)
	require.NoError(t, err)
	assert.Len(t, list, 2, "one latest per (device, metric)")
}
