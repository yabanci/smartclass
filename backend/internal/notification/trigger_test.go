package notification_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/notification"
	"smartclass/internal/notification/notificationtest"
	"smartclass/internal/realtime"
)

type staticMembers struct{ ids []uuid.UUID }

func (s staticMembers) Members(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return s.ids, nil
}

func TestEngine_SensorReading_HighTemperature(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "temperature", 35, "C")

	list, err := svc.List(context.Background(), member, true, 0, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, notification.TypeWarning, list[0].Type)
	assert.Contains(t, list[0].Message, "High temperature")
}

func TestEngine_SensorReading_Normal_NoAlert(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "temperature", 22, "C")

	list, err := svc.List(context.Background(), member, false, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestEngine_DeviceOffline_Notifies(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	a, b := uuid.New(), uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{a, b}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	eng.OnDeviceStateChange(context.Background(), uuid.New(), uuid.New(), "Lamp", false, "off")

	for _, u := range []uuid.UUID{a, b} {
		list, err := svc.List(context.Background(), u, true, 0, 0)
		require.NoError(t, err)
		require.Len(t, list, 1, "member %s should have 1 notif", u)
		assert.Contains(t, list[0].Message, "Lamp")
	}

	eng.OnDeviceStateChange(context.Background(), uuid.New(), uuid.New(), "Lamp", true, "on")
	list, err := svc.List(context.Background(), a, false, 0, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1, "coming back online should not add another notif")
}

func TestService_MarkRead(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	user := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{user}}, realtime.Noop{})

	n, err := svc.CreateForUser(context.Background(), notification.Input{
		UserID: user, Type: notification.TypeInfo, Title: "hi", Message: "hello",
	})
	require.NoError(t, err)

	count, _ := svc.CountUnread(context.Background(), user)
	assert.Equal(t, 1, count)

	require.NoError(t, svc.MarkRead(context.Background(), user, n.ID))
	count, _ = svc.CountUnread(context.Background(), user)
	assert.Equal(t, 0, count)
}
