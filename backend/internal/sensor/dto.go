package sensor

import "time"

type ReadingDTO struct {
	ID         int64          `json:"id,omitempty"`
	DeviceID   string         `json:"deviceId"`
	Metric     string         `json:"metric"`
	Value      float64        `json:"value"`
	Unit       string         `json:"unit,omitempty"`
	RecordedAt time.Time      `json:"recordedAt"`
	Raw        map[string]any `json:"raw,omitempty"`
}

func ToDTO(r Reading) ReadingDTO {
	return ReadingDTO{
		ID: r.ID, DeviceID: r.DeviceID.String(),
		Metric: string(r.Metric), Value: r.Value, Unit: r.Unit,
		RecordedAt: r.RecordedAt, Raw: r.Raw,
	}
}

type IngestItemRequest struct {
	DeviceID   string         `json:"deviceId" validate:"required,uuid"`
	Metric     string         `json:"metric" validate:"required"`
	Value      float64        `json:"value"`
	Unit       string         `json:"unit,omitempty" validate:"max=20"`
	RecordedAt *time.Time     `json:"recordedAt,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

type IngestRequest struct {
	Readings []IngestItemRequest `json:"readings" validate:"required,min=1,max=500,dive"`
}

type IngestResponse struct {
	Accepted int `json:"accepted"`
}
