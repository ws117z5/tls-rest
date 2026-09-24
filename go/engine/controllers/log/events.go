package log

import (
	"encoding/json"

	"fmt"
	"os"
	"time"
)

type EventType string

const (
	EventTypeAuth   EventType = "auth"
	EventTypeError  EventType = "error"
	EventTypeSystem EventType = "system"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// EventLog fields are limited to what a real caller below actually sets;
// per-request logging lives in access_log, not here.
type EventLog struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      EventType              `json:"type"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Module    string                 `json:"module,omitempty"`
	Action    string                 `json:"action,omitempty"`
	UserID    *int                   `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Source    string                 `json:"source,omitempty"` // pkg/file.go:line
}

type EventLogger struct {
	writeToFile bool
	writeToDb   bool
	logFile     *os.File
}

var GlobalEventLogger *EventLogger

func init() {
	GlobalEventLogger = &EventLogger{writeToFile: true}
	GlobalEventLogger.initLogFile()
}

func (el *EventLogger) initLogFile() {
	if !el.writeToFile {
		return
	}

	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		return
	}

	filename := fmt.Sprintf("%s/events_%s.log", logDir, time.Now().Format("2006-01-02"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	el.logFile = file
}

// LogErrorWithID logs a 5xx error and returns its event ID, shown to callers as log_id.
func LogErrorWithID(message, module, errStr string) string {
	id := generateEventID()
	if GlobalEventLogger == nil {
		return id
	}
	GlobalEventLogger.writeEvent(EventLog{
		ID:        id,
		Timestamp: time.Now(),
		Type:      EventTypeError,
		Level:     LogLevelError,
		Message:   message,
		Module:    module,
		Error:     errStr,
	})
	return id
}

// LogAuthEvent: auth module, used for access_denied/authorization_failed.
func LogAuthEvent(action, message string, userID *int, sessionID string, success bool, data map[string]interface{}) {
	level := LogLevelInfo
	if !success {
		level = LogLevelWarn
	}

	event := EventLog{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeAuth,
		Level:     level,
		Message:   message,
		UserID:    userID,
		SessionID: sessionID,
		Action:    action,
		Data:      data,
	}
	if ip, ok := data["ip"].(string); ok {
		event.IPAddress = ip
	}

	GlobalEventLogger.writeEvent(event)
}

func LogSystemEvent(message string, level LogLevel, data map[string]interface{}) {
	event := EventLog{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeSystem,
		Level:     level,
		Message:   message,
		Data:      data,
	}

	GlobalEventLogger.writeEvent(event)
}

func (el *EventLogger) writeEvent(event EventLog) {
	if !allowed(string(event.Level)) {
		return
	}
	if event.Source == "" {
		event.Source = capture()
	}
	el.writeToConsole(event)
	emitEvent(el.toEvent(event))
	if el.writeToFile && el.logFile != nil {
		el.writeToLogFile(event)
	}
	if el.writeToDb {
		el.writeToDatabase(event)
	}
}

func (el *EventLogger) toEvent(e EventLog) Event {
	return Event{
		Time:    e.Timestamp,
		Level:   string(e.Level),
		Module:  e.Module,
		Message: e.Message,
		Data:    e.Data,
	}
}

// persist: log.go's LOG_STORE sink, file/db only (console already printed there).
func persist(ev Event) {
	if GlobalEventLogger == nil || !allowed(ev.Level) {
		return
	}
	e := EventLog{
		ID:        generateEventID(),
		Timestamp: ev.Time,
		Type:      EventTypeSystem,
		Level:     LogLevel(ev.Level),
		Message:   ev.Message,
		Module:    ev.Module,
		Data:      ev.Data,
		Source:    ev.Source,
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if GlobalEventLogger.writeToFile && GlobalEventLogger.logFile != nil {
		GlobalEventLogger.writeToLogFile(e)
	}
	if GlobalEventLogger.writeToDb {
		GlobalEventLogger.writeToDatabase(e)
	}
}

func (el *EventLogger) writeToConsole(event EventLog) {
	tag := event.Module
	if tag == "" {
		tag = string(event.Type)
	}
	printLine(levelByName(string(event.Level)), tag, event.Source, event.Message)
}

func (el *EventLogger) writeToLogFile(event EventLog) {
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Failed to marshal event: %v\n", err)
		return
	}

	_, err = el.logFile.Write(append(jsonBytes, '\n'))
	if err != nil {
		fmt.Printf("Failed to write to log file: %v\n", err)
	}
}

// writeToDatabase: column set must match engine/modules/logs' fieldset.
func (el *EventLogger) writeToDatabase(event EventLog) {
	if db == nil {
		return
	}
	row := map[string]interface{}{
		"event_id":   event.ID,
		"created":    event.Timestamp,
		"type":       string(event.Type),
		"level":      string(dbLevel(event.Level)),
		"message":    event.Message,
		"module":     nullIfEmpty(event.Module),
		"action":     nullIfEmpty(event.Action),
		"session_id": nullIfEmpty(event.SessionID),
		"ip_address": nullIfEmpty(event.IPAddress),
		"error":      nullIfEmpty(event.Error),
		"source":     nullIfEmpty(event.Source),
	}
	if event.UserID != nil {
		row["user_id"] = *event.UserID
	}
	if _, err := db.InsertRow("logs", row); err != nil {
		fmt.Printf("event db write failed: %v\n", err)
	}
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func EnableDatabaseLogging(enable bool) {
	GlobalEventLogger.writeToDb = enable
}

func EnableFileLogging(enable bool) {
	GlobalEventLogger.writeToFile = enable
	if enable && GlobalEventLogger.logFile == nil {
		GlobalEventLogger.initLogFile()
	}
}

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
