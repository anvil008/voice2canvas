package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// The `level` key and its uppercase values are a contract with the log
// pipeline: Alloy reads them off stderr to label this container's lines.
func TestNewLogHandlerEmitsJSONLevels(t *testing.T) {
	var out bytes.Buffer
	handler, badLevel := newLogHandler(&out, "", "")
	if badLevel != "" {
		t.Fatalf("unexpected bad level %q", badLevel)
	}
	slog.New(handler).Error("boom", "error", "detail")

	var record map[string]any
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, out.String())
	}
	if record["level"] != "ERROR" {
		t.Fatalf("level = %v, want ERROR", record["level"])
	}
	if record["msg"] != "boom" || record["error"] != "detail" {
		t.Fatalf("unexpected record %v", record)
	}
}

func TestNewLogHandlerHonorsFormatAndLevel(t *testing.T) {
	var out bytes.Buffer
	handler, badLevel := newLogHandler(&out, "text", "debug")
	if badLevel != "" {
		t.Fatalf("unexpected bad level %q", badLevel)
	}
	slog.New(handler).Debug("hello")
	if !strings.Contains(out.String(), "level=DEBUG msg=hello") {
		t.Fatalf("unexpected text output %q", out.String())
	}

	if _, badLevel := newLogHandler(&out, "", "screaming"); badLevel != "screaming" {
		t.Fatalf("badLevel = %q, want screaming", badLevel)
	}
}
