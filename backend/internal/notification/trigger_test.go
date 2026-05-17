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

// TestEngine_Cooldown verifies that repeated alerts for the same device+rule
// within the cooldown window are suppressed so users don't get flooded.
func TestEngine_Cooldown_SuppressesDuplicates(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	cid := uuid.New()
	did := uuid.New()
	ctx := context.Background()

	// First reading above threshold — should fire.
	eng.OnSensorReading(ctx, cid, did, "temperature", 35, "C")
	// Second reading still above threshold within cooldown — must be suppressed.
	eng.OnSensorReading(ctx, cid, did, "temperature", 36, "C")
	// Third reading — still suppressed.
	eng.OnSensorReading(ctx, cid, did, "temperature", 37, "C")

	list, err := svc.List(ctx, member, false, 50, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1, "only the first alert should fire; subsequent ones are in cooldown")
}

// TestEngine_Cooldown_DifferentDevicesFireIndependently verifies that the
// cooldown is per-device: a second device above threshold still fires even if
// the first device's cooldown is active.
func TestEngine_Cooldown_DifferentDevicesFireIndependently(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	cid := uuid.New()
	dev1, dev2 := uuid.New(), uuid.New()
	ctx := context.Background()

	eng.OnSensorReading(ctx, cid, dev1, "temperature", 35, "C")
	eng.OnSensorReading(ctx, cid, dev2, "temperature", 35, "C")

	list, err := svc.List(ctx, member, false, 50, 0)
	require.NoError(t, err)
	assert.Len(t, list, 2, "each device should fire its own alert")
}

// TestEngine_ThresholdBoundary_ExactlyAt_NoFire verifies the threshold rule
// is `>` not `>=`. A reading exactly at the threshold must NOT alert —
// otherwise every borderline measurement spams the user.
func TestEngine_ThresholdBoundary_ExactlyAt_NoFire(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	// DefaultRules.TemperatureHighC == 30 (per package); equal must not fire.
	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "temperature", 30.0, "C")

	list, err := svc.List(context.Background(), member, false, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, list,
		"a reading exactly at the threshold must NOT fire — the rule is strictly greater (>), not >=, "+
			"otherwise every borderline reading alerts")
}

// TestEngine_ThresholdBoundary_JustAbove_Fires verifies the smallest possible
// reading above the threshold is enough to trigger — no hidden epsilon.
func TestEngine_ThresholdBoundary_JustAbove_Fires(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "temperature", 30.001, "C")

	list, err := svc.List(context.Background(), member, false, 50, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1,
		"the smallest possible reading above the threshold must fire — that's what the alert is for")
}

// TestEngine_LowTemperatureFires covers the symmetric `<` rule for cold
// readings. Without this we'd only catch overheating, not freezing.
func TestEngine_LowTemperatureFires(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	// DefaultRules.TemperatureLowC == 14 in DefaultRules (a chilly classroom).
	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "temperature", 5.0, "C")

	list, err := svc.List(context.Background(), member, false, 50, 0)
	require.NoError(t, err)
	require.Len(t, list, 1, "5°C is well below the cold threshold and must fire")
	assert.Contains(t, list[0].Message, "Low temperature")
}

// TestEngine_HighHumidityFires covers the humidity rule. DefaultRules sets
// HumidityHigh = 80; readings above must alert.
func TestEngine_HighHumidityFires(t *testing.T) {
	repo := notificationtest.NewMemRepo()
	member := uuid.New()
	svc := notification.NewService(repo, staticMembers{ids: []uuid.UUID{member}}, realtime.Noop{})
	eng := notification.NewEngine(svc, notification.DefaultRules())

	eng.OnSensorReading(context.Background(), uuid.New(), uuid.New(), "humidity", 90.0, "%")

	list, err := svc.List(context.Background(), member, false, 50, 0)
	require.NoError(t, err)
	require.Len(t, list, 1, "90%% humidity is above the threshold and must fire")
	assert.Contains(t, list[0].Message, "High humidity")
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
