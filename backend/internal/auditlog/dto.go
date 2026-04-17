package auditlog

import "time"

type DTO struct {
	ID         int64          `json:"id"`
	ActorID    *string        `json:"actorId,omitempty"`
	EntityType string         `json:"entityType"`
	EntityID   *string        `json:"entityId,omitempty"`
	Action     string         `json:"action"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

func ToDTO(e Entry) DTO {
	d := DTO{
		ID: e.ID, EntityType: string(e.EntityType),
		Action: string(e.Action), Metadata: e.Metadata, CreatedAt: e.CreatedAt,
	}
	if e.ActorID != nil {
		s := e.ActorID.String()
		d.ActorID = &s
	}
	if e.EntityID != nil {
		s := e.EntityID.String()
		d.EntityID = &s
	}
	return d
}
