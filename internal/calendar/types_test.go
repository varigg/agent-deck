package calendar

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent_TimeUntilLabel(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		startsAt time.Time
		want     string
	}{
		{"future_minutes", now.Add(12 * time.Minute), "12m"},
		{"future_hours", now.Add(90 * time.Minute), "1h30m"},
		{"past_event", now.Add(-5 * time.Minute), "now"},
		{"imminent", now.Add(30 * time.Second), "now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Title: "test", StartsAt: tt.startsAt}
			assert.Equal(t, tt.want, e.TimeUntilLabel())
		})
	}
}

func TestEvent_Urgency(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		startsAt time.Time
		want     Urgency
	}{
		{"critical", now.Add(3 * time.Minute), UrgencyCritical},
		{"warning", now.Add(10 * time.Minute), UrgencyWarning},
		{"normal", now.Add(45 * time.Minute), UrgencyNormal},
		{"in_progress", now.Add(-5 * time.Minute), UrgencyCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Title: "test", StartsAt: tt.startsAt}
			assert.Equal(t, tt.want, e.Urgency())
		})
	}
}

func TestEvent_StartsInMinutes(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		startsAt time.Time
		want     int
	}{
		{"future", now.Add(12*time.Minute + 30*time.Second), 13}, // ceiling
		{"exactly_15m", now.Add(15 * time.Minute), 15},
		{"imminent", now.Add(30 * time.Second), 1}, // ceiling: < 1 min rounds to 1
		{"past", now.Add(-5 * time.Minute), 0},
		{"in_progress", now.Add(-1 * time.Second), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Event{Title: "test", StartsAt: tt.startsAt}
			assert.Equal(t, tt.want, e.StartsInMinutes())
		})
	}
}

func TestParseEventsResponse(t *testing.T) {
	raw := `{
		"items": [
			{
				"summary": "Standup",
				"status": "confirmed",
				"start": {"dateTime": "2026-03-25T10:00:00-07:00"},
				"end": {"dateTime": "2026-03-25T10:15:00-07:00"}
				},
			{
				"summary": "All-day event",
				"status": "confirmed",
				"start": {"date": "2026-03-25"},
				"end": {"date": "2026-03-26"}
			}
		]
	}`
	var resp eventsListResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, "Standup", resp.Items[0].Summary)
	assert.Equal(t, "2026-03-25", resp.Items[1].Start.Date)
}
