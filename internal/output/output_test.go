package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zyrophix/lethe/internal/risk"
)

func TestTextWriterInfo(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Info("shell", "cleaning histories")
	if !strings.Contains(buf.String(), "[*] [shell] cleaning histories") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestTextWriterSuccess(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Success("shell", "~/.bash_history", "truncated", risk.RiskSafe)
	if !strings.Contains(buf.String(), "[+] [shell] truncated ~/.bash_history (safe)") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestTextWriterDebug(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, debug: true, useColor: false}
	w.Debug("shell", "debug msg")
	if !strings.Contains(buf.String(), "[D] [shell] debug msg") {
		t.Errorf("unexpected output: %q", buf.String())
	}

	buf.Reset()
	w.debug = false
	w.Debug("shell", "should not appear")
	if buf.String() != "" {
		t.Errorf("debug should be suppressed: %q", buf.String())
	}
}

func TestJSONWriterInfo(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, debug: true}
	w.Info("shell", "cleaning histories")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Level != LevelInfo || entry.Module != "shell" || entry.Message != "cleaning histories" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestJSONWriterSuccess(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, debug: true}
	w.Success("shell", "~/.bash_history", "truncated", risk.RiskSafe)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Level != LevelSuccess || entry.Risk != "safe" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestJSONWriterSummary(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, debug: true}
	w.Summary(10, 1, 2, 3, time.Second, true)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Message != "cleaning_complete" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestTextWriterWarning(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Warning("risky operation")
	if !strings.Contains(buf.String(), "[!] risky operation") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestTextWriterWarningColor(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: true}
	w.Warning("risky operation")
	if !strings.Contains(buf.String(), "\033[33m") {
		t.Errorf("expected color codes: %q", buf.String())
	}
}

func TestTextWriterError(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Error("something failed")
	if !strings.Contains(buf.String(), "[-] something failed") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestTextWriterErrorColor(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: true}
	w.Error("something failed")
	if !strings.Contains(buf.String(), "\033[31m") {
		t.Errorf("expected color codes: %q", buf.String())
	}
}

func TestTextWriterSuccessColor(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: true}
	w.Success("shell", "~/.bash_history", "truncated", risk.RiskRisky)
	if !strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected color codes: %q", buf.String())
	}
}

func TestTextWriterSummary(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Summary(5, 1, 2, 3, 1500*time.Millisecond, false)

	s := buf.String()
	for _, want := range []string{"Cleaned:   5", "Failed:    1", "Skipped:   2", "Backed up: 3", "Duration:  1.5s"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary missing %q: %s", want, s)
		}
	}
	if strings.Contains(s, "dry run") {
		t.Errorf("should not mention dry run: %s", s)
	}
}

func TestTextWriterSummaryDryRun(t *testing.T) {
	var buf bytes.Buffer
	w := &TextWriter{out: &buf, useColor: false}
	w.Summary(0, 0, 10, 0, time.Second, true)
	if !strings.Contains(buf.String(), "This was a dry run") {
		t.Errorf("expected dry run note: %q", buf.String())
	}
}

func TestTextWriterAuditLog(t *testing.T) {
	logPath := t.TempDir() + "/audit.log"
	w := NewTextWriter(false, logPath)
	if w.auditLog == nil {
		t.Fatal("audit log should be opened")
	}
	w.Info("shell", "cleaning")
	w.Warning("careful")
	w.Success("shell", "a", "delete", risk.RiskSafe)
	w.Flush()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 audit entries, got %d: %s", len(lines), data)
	}
	for _, line := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid audit JSON: %v", err)
		}
		if e.Timestamp == "" {
			t.Errorf("entry missing timestamp: %+v", e)
		}
		if e.Level == LevelSuccess {
			if e.Artifact != "a" || e.Action != "delete" {
				t.Errorf("success entry missing artifact/action: %+v", e)
			}
		} else if e.Message == "" {
			t.Errorf("entry missing message: %+v", e)
		}
	}
}

func TestJSONWriterAuditLog(t *testing.T) {
	logPath := t.TempDir() + "/audit.json"
	w := NewJSONWriter(false, false, logPath)
	if w.auditLog == nil {
		t.Fatal("audit log should be opened")
	}
	w.Info("shell", "cleaning")
	w.Error("failed")
	w.Flush()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(lines))
	}
}

func TestJSONWriterWarningErrorDebug(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, debug: true}
	w.Warning("careful")
	w.Error("boom")
	w.Debug("shell", "dbg")

	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		entries = append(entries, e)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %v", entries)
	}
	if entries[0].Level != LevelWarning || entries[1].Level != LevelError || entries[2].Level != LevelDebug {
		t.Errorf("unexpected levels: %v", entries)
	}
}

func TestJSONWriterDebugSuppressed(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, debug: false}
	w.Debug("shell", "hidden")
	if buf.String() != "" {
		t.Errorf("debug should be suppressed: %q", buf.String())
	}
}

func TestJSONWriterDryRunFlag(t *testing.T) {
	var buf bytes.Buffer
	w := &JSONWriter{out: &buf, dryRun: true}
	w.Info("shell", "preview")

	var e Entry
	if err := json.Unmarshal(buf.Bytes(), &e); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !e.DryRun {
		t.Error("dry_run flag should be set")
	}
}

func TestSupportsColor(t *testing.T) {
	origTerm, origColor := os.Getenv("TERM"), os.Getenv("COLORTERM")
	defer func() {
		_ = os.Setenv("TERM", origTerm)
		_ = os.Setenv("COLORTERM", origColor)
	}()

	_ = os.Setenv("COLORTERM", "")
	_ = os.Setenv("TERM", "xterm-256color")
	if !supportsColor() {
		t.Error("xterm should support color")
	}

	_ = os.Setenv("TERM", "dumb")
	if supportsColor() {
		t.Error("dumb terminal should not support color")
	}
}
