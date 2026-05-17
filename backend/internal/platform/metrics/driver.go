package metrics

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// TrackDriver wraps a single driver command call. Drivers call it with the
// driver name, the command type, and a closure that performs the actual HTTP
// request to the device/integration. It increments the calls counter with
// the right `result` label, observes the duration histogram, and returns the
// inner error verbatim — instrumentation never silences errors.
func TrackDriver(ctx context.Context, driver, command string, fn func(ctx context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	result := "ok"
	if err != nil {
		result = "err"
	}
	DriverCalls.WithLabelValues(driver, command, result).Inc()
	DriverDuration.WithLabelValues(driver, command).Observe(time.Since(start).Seconds())
	return err
}

// TrackHass wraps a single Home Assistant API call (StartFlow, StepFlow,
// AbortFlow, requestJSON). Same shape as TrackDriver, different metric.
func TrackHass(ctx context.Context, op string, fn func(ctx context.Context) error) error {
	return TrackHassLog(ctx, op, nil, fn)
}

// hassSlowThreshold is the duration above which TrackHassLog emits a WARN.
const hassSlowThreshold = 5 * time.Second

// TrackHassLog is like TrackHass but additionally emits a WARN via logger
// (when non-nil) if the call exceeds hassSlowThreshold (5 s). Pass a nil
// logger to suppress slow-call logging (identical to TrackHass).
func TrackHassLog(ctx context.Context, op string, logger *zap.Logger, fn func(ctx context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	dur := time.Since(start)
	result := "ok"
	if err != nil {
		result = "err"
	}
	HassCalls.WithLabelValues(op, result).Inc()
	HassDuration.WithLabelValues(op).Observe(dur.Seconds())
	if logger != nil && dur > hassSlowThreshold {
		logger.Warn("hass: slow call", zap.String("op", op), zap.Duration("duration", dur))
	}
	return err
}
