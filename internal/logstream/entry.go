package logstream

import "time"

// LogEntry is a single captured log record.
type LogEntry struct {
	Time      time.Time         `json:"time"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Component string            `json:"component"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}
