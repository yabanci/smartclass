package sensor

import (
	"time"

	"github.com/google/uuid"
)

type Metric string

const (
	MetricTemperature Metric = "temperature"
	MetricHumidity    Metric = "humidity"
	MetricMotion      Metric = "motion"
	MetricAirQuality  Metric = "air_quality"
	MetricEnergy      Metric = "energy"
	MetricDoor        Metric = "door"
)

func (m Metric) Valid() bool {
	switch m {
	case MetricTemperature, MetricHumidity, MetricMotion, MetricAirQuality, MetricEnergy, MetricDoor:
		return true
	}
	return false
}

type Reading struct {
	ID         int64
	DeviceID   uuid.UUID
	Metric     Metric
	Value      float64
	Unit       string
	RecordedAt time.Time
	Raw        map[string]any
}
