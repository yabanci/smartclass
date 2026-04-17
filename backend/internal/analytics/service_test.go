package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/analytics"
	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/user"
)

type fakeRepo struct {
	series []analytics.TimePoint
	usage  []analytics.DeviceUsage
	energy float64
}

func (f *fakeRepo) SensorSeries(_ context.Context, _ uuid.UUID, _ string, _ analytics.Bucket, _, _ time.Time) ([]analytics.TimePoint, error) {
	return f.series, nil
}
func (f *fakeRepo) DeviceUsage(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]analytics.DeviceUsage, error) {
	return f.usage, nil
}
func (f *fakeRepo) EnergyTotal(_ context.Context, _ uuid.UUID, _, _ time.Time) (float64, error) {
	return f.energy, nil
}

func setup(t *testing.T) (*analytics.Service, classroom.Principal, uuid.UUID, *fakeRepo) {
	t.Helper()
	clsSvc := classroom.NewService(classroomtest.NewMemRepo())
	ownerID := uuid.New()
	p := classroom.Principal{UserID: ownerID, Role: user.RoleTeacher}
	cls, err := clsSvc.Create(context.Background(), classroom.CreateInput{Name: "R", CreatedBy: ownerID})
	require.NoError(t, err)
	repo := &fakeRepo{}
	return analytics.NewService(repo, clsSvc), p, cls.ID, repo
}

func TestSensorSeries_InvalidBucket(t *testing.T) {
	svc, p, cid, _ := setup(t)
	_, err := svc.SensorSeries(context.Background(), p, cid, "temperature", "millisecond", time.Time{}, time.Time{})
	require.ErrorIs(t, err, analytics.ErrInvalidBucket)
}

func TestSensorSeries_OK(t *testing.T) {
	svc, p, cid, repo := setup(t)
	repo.series = []analytics.TimePoint{{Avg: 22, Min: 20, Max: 24, Count: 10}}
	list, err := svc.SensorSeries(context.Background(), p, cid, "temperature", analytics.BucketHour, time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.InDelta(t, 22, list[0].Avg, 0.001)
}

func TestDeviceUsage_AuthDenied(t *testing.T) {
	svc, _, cid, _ := setup(t)
	stranger := classroom.Principal{UserID: uuid.New(), Role: user.RoleTeacher}
	_, err := svc.DeviceUsage(context.Background(), stranger, cid, time.Time{}, time.Time{})
	require.ErrorIs(t, err, classroom.ErrForbidden)
}

func TestEnergy_OK(t *testing.T) {
	svc, p, cid, repo := setup(t)
	repo.energy = 42.5
	total, err := svc.EnergyTotal(context.Background(), p, cid, time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.InDelta(t, 42.5, total, 0.001)
}
