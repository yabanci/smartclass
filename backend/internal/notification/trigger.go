package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Engine is a simple rule engine that emits notifications on incoming domain
// events. Its method signatures are intentionally primitive so that callers
// (device.Service, sensor.Service) depend only on small interfaces, not on
// this package's types.
type Engine struct {
	svc   *Service
	rules Rules
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
	return &Engine{svc: svc, rules: rules}
}

func (e *Engine) OnDeviceStateChange(ctx context.Context, classroomID, deviceID uuid.UUID, name string, online bool, _ string) {
	if e == nil || e.svc == nil || online {
		return
	}
	_, _ = e.svc.CreateForClassroom(ctx, classroomID, Input{
		Type:    TypeWarning,
		Title:   "Device offline",
		Message: fmt.Sprintf("Device %q went offline.", name),
		Metadata: map[string]any{
			"deviceId": deviceID.String(),
			"rule":     "device_offline",
		},
	})
}

func (e *Engine) OnSensorReading(ctx context.Context, classroomID, deviceID uuid.UUID, metric string, value float64, _ string) {
	if e == nil || e.svc == nil {
		return
	}
	switch metric {
	case "temperature":
		if value > e.rules.TemperatureHighC {
			e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("High temperature: %.1f°C (threshold %.1f)", value, e.rules.TemperatureHighC), "temperature_high")
		} else if value < e.rules.TemperatureLowC {
			e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("Low temperature: %.1f°C (threshold %.1f)", value, e.rules.TemperatureLowC), "temperature_low")
		}
	case "humidity":
		if value > e.rules.HumidityHigh {
			e.warn(ctx, classroomID, deviceID, metric, value, fmt.Sprintf("High humidity: %.0f%% (threshold %.0f%%)", value, e.rules.HumidityHigh), "humidity_high")
		}
	}
}

func (e *Engine) warn(ctx context.Context, classroomID, deviceID uuid.UUID, metric string, value float64, message, rule string) {
	_, _ = e.svc.CreateForClassroom(ctx, classroomID, Input{
		Type:    TypeWarning,
		Title:   "Environment alert",
		Message: message,
		Metadata: map[string]any{
			"deviceId": deviceID.String(),
			"metric":   metric,
			"value":    value,
			"rule":     rule,
		},
	})
}
