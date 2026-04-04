package calendar

import (
	"fmt"
	"time"
)

// Urgency levels for calendar events, driving tmux color coding.
type Urgency int

const (
	UrgencyNormal   Urgency = iota // > 15 minutes away
	UrgencyWarning                 // 5–15 minutes away
	UrgencyCritical                // < 5 minutes or in progress
)

// Event is the domain representation of an upcoming calendar event.
type Event struct {
	Title      string    `json:"title"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	CalendarID string    `json:"-"`
}

// TimeUntilLabel returns a human-readable duration string like "12m" or "1h30m".
// Returns "now" for events starting within 1 minute or already in progress.
// Uses ceiling rounding so "12m" stays "12m" until exactly 11m0s remains.
func (e Event) TimeUntilLabel() string {
	d := time.Until(e.StartsAt)
	if d < time.Minute {
		return "now"
	}
	// Ceiling division to nearest minute
	mins := int((d + time.Minute - 1) / time.Minute)
	h := mins / 60
	m := mins % 60
	if h > 0 {
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// Urgency returns the urgency level based on time until the event starts.
func (e Event) Urgency() Urgency {
	d := time.Until(e.StartsAt)
	switch {
	case d < 5*time.Minute:
		return UrgencyCritical
	case d < 15*time.Minute:
		return UrgencyWarning
	default:
		return UrgencyNormal
	}
}

// StartsInMinutes returns how many minutes until the event starts, rounded up.
// Returns 0 for events that have already started or start within the next minute.
func (e Event) StartsInMinutes() int {
	d := time.Until(e.StartsAt)
	if d <= 0 {
		return 0
	}
	// Ceiling division: consistent with TimeUntilLabel's rounding.
	return int((d + time.Minute - 1) / time.Minute)
}

// MeetingInfo is a compact summary of an upcoming event suitable for JSON
// serialisation (e.g. agent-deck status --json) and external consumers like
// the conductor heartbeat.
type MeetingInfo struct {
	Title           string `json:"title"`
	StartsInMinutes int    `json:"starts_in_minutes"`
}

// NextMeeting returns a MeetingInfo for the first event in the slice, or nil
// when the slice is empty. It is the caller's responsibility to pass a slice
// that is already sorted by start time.
func NextMeeting(events []Event) *MeetingInfo {
	if len(events) == 0 {
		return nil
	}
	e := events[0]
	return &MeetingInfo{
		Title:           e.Title,
		StartsInMinutes: e.StartsInMinutes(),
	}
}

// --- Google Calendar API response structs (minimal) ---

// eventsListResponse mirrors the subset of fields we need from
// GET /calendar/v3/calendars/{id}/events
type eventsListResponse struct {
	Items []eventResource `json:"items"`
}

type eventResource struct {
	Summary string        `json:"summary"`
	Status  string        `json:"status"`
	Start   eventDateTime `json:"start"`
	End     eventDateTime `json:"end"`
}

type eventDateTime struct {
	DateTime string `json:"dateTime"` // RFC3339 for timed events
	Date     string `json:"date"`     // YYYY-MM-DD for all-day events
}
