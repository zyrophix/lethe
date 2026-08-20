package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lethe/lethe/internal/risk"
)

type Level string

const (
	LevelInfo    Level = "info"
	LevelSuccess Level = "success"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelDebug   Level = "debug"
)

type Entry struct {
	Timestamp string `json:"ts,omitempty"`
	Level     Level  `json:"level"`
	Module    string `json:"module,omitempty"`
	Artifact  string `json:"artifact,omitempty"`
	Action    string `json:"action,omitempty"`
	Risk      string `json:"risk,omitempty"`
	Message   string `json:"msg"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type Writer interface {
	Info(module, msg string)
	Success(module, artifact, action string, riskLevel risk.RiskLevel)
	Warning(msg string)
	Error(msg string)
	Debug(module, msg string)
	Summary(cleaned, failed, skipped, backedUp int, duration time.Duration, dryRun bool)
	Flush()
}

type TextWriter struct {
	mu       sync.Mutex
	out      io.Writer
	debug    bool
	auditLog *os.File
	useColor bool
}

func NewTextWriter(debug bool, auditLogPath string) *TextWriter {
	w := &TextWriter{
		out:      os.Stdout,
		debug:    debug,
		useColor: os.Getenv("NO_COLOR") == "" && supportsColor(),
	}
	if auditLogPath != "" {
		f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			w.auditLog = f
		}
	}
	return w
}

func (w *TextWriter) Info(module, msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.out, "[*] [%s] %s\n", module, msg)
	w.writeAudit(LevelInfo, module, "", "", "", msg)
}

func (w *TextWriter) Success(module, artifact, action string, riskLevel risk.RiskLevel) {
	w.mu.Lock()
	defer w.mu.Unlock()
	riskStr := riskLevel.String()
	if w.useColor {
		color := riskLevel.Color()
		fmt.Fprintf(w.out, "[+] [%s] %s %s (%s%s%s)\n", module, action, artifact, color, riskStr, riskLevel.Reset())
	} else {
		fmt.Fprintf(w.out, "[+] [%s] %s %s (%s)\n", module, action, artifact, riskStr)
	}
	w.writeAudit(LevelSuccess, module, artifact, action, riskStr, "")
}

func (w *TextWriter) Warning(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.useColor {
		fmt.Fprintf(w.out, "\033[33m[!] %s\033[0m\n", msg)
	} else {
		fmt.Fprintf(w.out, "[!] %s\n", msg)
	}
	w.writeAudit(LevelWarning, "", "", "", "", msg)
}

func (w *TextWriter) Error(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.useColor {
		fmt.Fprintf(w.out, "\033[31m[-] %s\033[0m\n", msg)
	} else {
		fmt.Fprintf(w.out, "[-] %s\n", msg)
	}
	w.writeAudit(LevelError, "", "", "", "", msg)
}

func (w *TextWriter) Debug(module, msg string) {
	if !w.debug {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.out, "[D] [%s] %s\n", module, msg)
	w.writeAudit(LevelDebug, module, "", "", "", msg)
}

func (w *TextWriter) Summary(cleaned, failed, skipped, backedUp int, duration time.Duration, dryRun bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintln(w.out)
	fmt.Fprintln(w.out, "================================================")
	fmt.Fprintln(w.out, " Cleaning Complete!")
	fmt.Fprintln(w.out, "================================================")
	fmt.Fprintf(w.out, "  Cleaned:   %d\n", cleaned)
	fmt.Fprintf(w.out, "  Failed:    %d\n", failed)
	fmt.Fprintf(w.out, "  Skipped:   %d\n", skipped)
	fmt.Fprintf(w.out, "  Backed up: %d\n", backedUp)
	fmt.Fprintf(w.out, "  Duration:  %s\n", duration.Round(time.Millisecond))
	if dryRun {
		if w.useColor {
			fmt.Fprintln(w.out, "\n\033[33m  This was a dry run. No actual changes were made.\033[0m")
		} else {
			fmt.Fprintln(w.out, "\n  This was a dry run. No actual changes were made.")
		}
	}
	fmt.Fprintln(w.out)
}

func (w *TextWriter) Flush() {
	if w.auditLog != nil {
		w.auditLog.Close()
	}
}

func (w *TextWriter) writeAudit(level Level, module, artifact, action, riskStr, msg string) {
	if w.auditLog == nil {
		return
	}
	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Module:    module,
		Artifact:  artifact,
		Action:    action,
		Risk:      riskStr,
		Message:   msg,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	w.auditLog.Write(append(data, '\n'))
}

type JSONWriter struct {
	mu       sync.Mutex
	out      io.Writer
	auditLog *os.File
	debug    bool
	dryRun   bool
}

func NewJSONWriter(debug bool, dryRun bool, auditLogPath string) *JSONWriter {
	w := &JSONWriter{
		out:    os.Stdout,
		debug:  debug,
		dryRun: dryRun,
	}
	if auditLogPath != "" {
		f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			w.auditLog = f
		}
	}
	return w
}

func (w *JSONWriter) writeEntry(entry Entry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dryRun {
		entry.DryRun = true
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	w.out.Write(append(data, '\n'))
	if w.auditLog != nil {
		w.auditLog.Write(append(data, '\n'))
	}
}

func (w *JSONWriter) Info(module, msg string) {
	w.writeEntry(Entry{Timestamp: now(), Level: LevelInfo, Module: module, Message: msg})
}

func (w *JSONWriter) Success(module, artifact, action string, riskLevel risk.RiskLevel) {
	w.writeEntry(Entry{Timestamp: now(), Level: LevelSuccess, Module: module, Artifact: artifact, Action: action, Risk: riskLevel.String()})
}

func (w *JSONWriter) Warning(msg string) {
	w.writeEntry(Entry{Timestamp: now(), Level: LevelWarning, Message: msg})
}

func (w *JSONWriter) Error(msg string) {
	w.writeEntry(Entry{Timestamp: now(), Level: LevelError, Message: msg})
}

func (w *JSONWriter) Debug(module, msg string) {
	if w.debug {
		w.writeEntry(Entry{Timestamp: now(), Level: LevelDebug, Module: module, Message: msg})
	}
}

func (w *JSONWriter) Summary(cleaned, failed, skipped, backedUp int, duration time.Duration, dryRun bool) {
	w.writeEntry(Entry{
		Timestamp: now(),
		Level:     LevelInfo,
		Message:   "cleaning_complete",
		Action:    fmt.Sprintf("cleaned=%d failed=%d skipped=%d backed_up=%d duration=%s dry_run=%v", cleaned, failed, skipped, backedUp, duration, dryRun),
	})
}

func (w *JSONWriter) Flush() {
	if w.auditLog != nil {
		w.auditLog.Close()
	}
}

func now() string {
	return time.Now().Format(time.RFC3339Nano)
}

func supportsColor() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.Contains(term, "color") || strings.Contains(term, "xterm") || os.Getenv("COLORTERM") != ""
}
