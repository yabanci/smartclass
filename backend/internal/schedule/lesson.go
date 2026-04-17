package schedule

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DayOfWeek int

const (
	Monday    DayOfWeek = 1
	Tuesday   DayOfWeek = 2
	Wednesday DayOfWeek = 3
	Thursday  DayOfWeek = 4
	Friday    DayOfWeek = 5
	Saturday  DayOfWeek = 6
	Sunday    DayOfWeek = 7
)

func (d DayOfWeek) Valid() bool { return d >= Monday && d <= Sunday }

func FromTime(t time.Time) DayOfWeek {
	wd := t.Weekday()
	if wd == time.Sunday {
		return Sunday
	}
	return DayOfWeek(int(wd))
}

// TimeOfDay represents a clock time (HH:MM) stored as minutes since midnight.
// Using a compact integer avoids timezone/DST confusion for recurring weekly slots.
type TimeOfDay int

func ParseTimeOfDay(s string) (TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		t, err = time.Parse("15:04:05", s)
		if err != nil {
			return 0, fmt.Errorf("schedule: invalid time %q: %w", s, err)
		}
	}
	return TimeOfDay(t.Hour()*60 + t.Minute()), nil
}

func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60) }
func (t TimeOfDay) Valid() bool    { return t >= 0 && t < 24*60 }

type Lesson struct {
	ID          uuid.UUID
	ClassroomID uuid.UUID
	Subject     string
	TeacherID   *uuid.UUID
	DayOfWeek   DayOfWeek
	StartsAt    TimeOfDay
	EndsAt      TimeOfDay
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (l Lesson) Overlaps(other Lesson) bool {
	if l.DayOfWeek != other.DayOfWeek {
		return false
	}
	return l.StartsAt < other.EndsAt && other.StartsAt < l.EndsAt
}
