package observability

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	logger.Info("started", map[string]any{"addr": "127.0.0.1:18080"})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if entry["level"] != "info" {
		t.Fatalf("level = %q, want info", entry["level"])
	}
	if entry["message"] != "started" {
		t.Fatalf("message = %q, want started", entry["message"])
	}
	if entry["addr"] != "127.0.0.1:18080" {
		t.Fatalf("addr = %q, want 127.0.0.1:18080", entry["addr"])
	}
	if entry["time"] == "" {
		t.Fatal("time is empty")
	}
}
