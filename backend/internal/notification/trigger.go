package notification

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Engine is a simple rule engine that emits notifications on incoming domain
// events. Its method signatures are intentionally primitive so that callers
// (device.Service, sensor.Service) depend only on small interfaces, not on
// this package's types.
type Engine struct {
	svc      *Service
	rules    Rules
	cooldown time.Duration
	log      *zap.Logger

	mu        sync.Mutex
	lastAlert map[string]time.Time // key: classroomID:deviceID:rule
}

type Rules struct {
	TemperatureHighC float64
	TemperatureLowC  float64
	HumidityHigh     float64
}

func DefaultRules() Rules {
	return Rules{TemperatureHighC: 30, TemperatureLowC: 14, HumidityHigh: 80}
}

func NewEngine(svc *Service, rules Rules) *Engine {
	return &Engine{
		svc:       svc,
		rules:     rules,
		cooldown:  5 * time.Minute,
		log:       zap.NewNop(),
		lastAlert: make(map[string]time.Time),
	}
}

func (e *Engine) WithLogger(l *zap.Logger) *Engine {
	if l != nil {
		e.log = l
	}
	return e
}

// throttle returns true if an alert for this key should be suppressed because
// one was already fired within the cooldown window. Safe for concurrent use.
func (e *Engine) throttle(classroomID, deviceID uuid.UUID, rule string) bool {
	key := classroomID.String() + ":" + deviceID.String() + ":" + rule
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	if last, ok := e.lastAlert[key]; ok && now.Sub(last) < e.cooldown {
		return true
	}
	e.lastAlert[key] = now
	return false
}

func (e *Engine) OnDeviceStateChange(ctx context.Context, classroomID, deviceID uuid.UUID, name string, online bool, _ string) {
	if e == nil || e.svc == nil || online {
		return
	}
	if e.throttle(classroomID, deviceID, "device_offline") {
		return
	}
	if _, err := e.svc.CreateForClassroom(ctx, classroomID, Input{
		Type:    TypeWarning,
		Title:   "Device offline",
		Message: fmt.Sprintf("Device %q went offline.", name),
		Metadata: map[string]any{
			"deviceId": deviceID.String(),
			"rule":     "device_offline",
		},
	}); err != nil {
		e.log.Warn("trigger: failed to create device_offline notification", zap.Error(err))
	}
}

func (e *Engine) OnSensorReading(ctx context.Context, classroomID, deviceID uuid.UUID, metric string, value float64, _ string) {
	if e == nil || e.svc == nil {
		return
	}
	switch metric {
	case "temperature":
		if value > e.rules.TemperatureHighC {
			if !e.throttle(classroomID, deviceID, "temperature_high") {
				e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("High temperature: %.1f°C (threshold %.1f)", value, e.rules.TemperatureHighC), "temperature_high")
			}
		} else if value < e.rules.TemperatureLowC {
			if !e.throttle(classroomID, deviceID, "temperature_low") {
				e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("Low temperature: %.1f°C (threshold %.1f)", value, e.rules.TemperatureLowC), "temperature_low")
			}
		}
	case "humidity":
		if value > e.rules.HumidityHigh {
			if !e.throttle(classroomID, deviceID, "humidity_high") {
				e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("High humidity: %.0f%% (threshold %.0f%%)", value, e.rules.HumidityHigh), "humidity_high")
			}
		}
	}
}

func (e *Engine) warn(ctx context.Context, classroomID, deviceID uuid.UUID, metric string, value float64, message, rule string) {
	if _, err := e.svc.CreateForClassroom(ctx, classroomID, Input{
		Type:    TypeWarning,
		Title:   "Environment alert",
		Message: message,
		Metadata: map[string]any{
			"deviceId": deviceID.String(),
			"metric":   metric,
			"value":    value,
			"rule":     rule,
		},
	}); err != nil {
		e.log.Warn("trigger: failed to create environment alert", zap.String("rule", rule), zap.Error(err))
	}
}
