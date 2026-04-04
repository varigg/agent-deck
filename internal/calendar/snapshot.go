package calendar

import (
	"encoding/json"
	"errors"
	"os"
)

// WriteSnapshot persists events to a JSON file so other processes (e.g.
// agent-deck status --json) can read cached calendar data without making a
// live API call. The file is written atomically via a temp-file rename.
func WriteSnapshot(path string, events []Event) error {
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadSnapshot loads a previously written snapshot. Returns nil, nil when the
// file does not exist (TUI not running or calendar not configured).
func ReadSnapshot(path string) ([]Event, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
