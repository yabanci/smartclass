package notification

import "time"

type DTO struct {
	ID          string         `json:"id"`
	UserID      string         `json:"userId"`
	ClassroomID *string        `json:"classroomId,omitempty"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ReadAt      *time.Time     `json:"readAt,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

func ToDTO(n *Notification) DTO {
	var cid *string
	if n.ClassroomID != nil {
		s := n.ClassroomID.String()
		cid = &s
	}
	return DTO{
		ID: n.ID.String(), UserID: n.UserID.String(), ClassroomID: cid,
		Type: string(n.Type), Title: n.Title, Message: n.Message,
		Metadata: n.Metadata, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
	}
}
