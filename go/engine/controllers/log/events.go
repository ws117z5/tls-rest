package log

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// EventType represents different types of events
type EventType string

const (
	EventTypeRequest  EventType = "request"
	EventTypeResponse EventType = "response"
	EventTypeDatabase EventType = "database"
	EventTypeModule   EventType = "module"
	EventTypeAuth     EventType = "auth"
	EventTypeError    EventType = "error"
	EventTypeSystem   EventType = "system"
	EventTypeSession  EventType = "session"
)

// LogLevel represents the severity of the event
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// EventLog represents a structured log event
type EventLog struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Type       EventType              `json:"type"`
	Level      LogLevel               `json:"level"`
	Message    string                 `json:"message"`
	UserID     *int                   `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Module     string                 `json:"module,omitempty"`
	Action     string                 `json:"action,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Duration   *float64               `json:"duration_ms,omitempty"`
	StatusCode *int                   `json:"status_code,omitempty"`
	RequestURL string                 `json:"request_url,omitempty"`
	Method     string                 `json:"method,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Source     string                 `json:"source,omitempty"` // pkg/file.go:line
}

// EventLogger handles all event logging
type EventLogger struct {
	writeToFile bool
	writeToDb   bool
	logFile     *os.File
}

var GlobalEventLogger *EventLogger

func init() {
	GlobalEventLogger = &EventLogger{
		writeToFile: true,
		writeToDb:   false, // Can be enabled via config
	}
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

// LogErrorWithID logs an error event and returns its event ID, so callers can
// surface it to clients (e.g. in an error response's log_id) for later lookup in
// the logs module. Safe before the logger is initialized.
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

// LogEvent logs an event with the specified parameters
func LogEvent(eventType EventType, level LogLevel, message string, data map[string]interface{}) {
	GlobalEventLogger.LogEvent(eventType, level, message, data)
}

func (el *EventLogger) LogEvent(eventType EventType, level LogLevel, message string, data map[string]interface{}) {
	event := EventLog{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      eventType,
		Level:     level,
		Message:   message,
		Data:      data,
	}

	el.writeEvent(event)
}

// LogRequest logs HTTP request details
func LogRequest(r *http.Request, userID *int, sessionID string) string {
	requestID := generateEventID()

	data := map[string]interface{}{
		"headers":      r.Header,
		"query_params": r.URL.Query(),
	}

	event := EventLog{
		ID:         generateEventID(),
		Timestamp:  time.Now(),
		Type:       EventTypeRequest,
		Level:      LogLevelInfo,
		Message:    fmt.Sprintf("HTTP Request: %s %s", r.Method, r.URL.Path),
		UserID:     userID,
		SessionID:  sessionID,
		RequestID:  requestID,
		IPAddress:  getClientIP(r),
		UserAgent:  r.UserAgent(),
		RequestURL: r.URL.String(),
		Method:     r.Method,
		Data:       data,
	}

	GlobalEventLogger.writeEvent(event)
	return requestID
}

// LogResponse logs HTTP response details
func LogResponse(requestID string, statusCode int, duration float64, userID *int) {
	event := EventLog{
		ID:         generateEventID(),
		Timestamp:  time.Now(),
		Type:       EventTypeResponse,
		Level:      LogLevelInfo,
		Message:    fmt.Sprintf("HTTP Response: %d", statusCode),
		UserID:     userID,
		RequestID:  requestID,
		Duration:   &duration,
		StatusCode: &statusCode,
	}

	GlobalEventLogger.writeEvent(event)
}

// LogModuleEvent logs module-specific events
func LogModuleEvent(module, action, message string, userID *int, sessionID string, data map[string]interface{}) {
	event := EventLog{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeModule,
		Level:     LogLevelInfo,
		Message:   message,
		UserID:    userID,
		SessionID: sessionID,
		Module:    module,
		Action:    action,
		Data:      data,
	}

	GlobalEventLogger.writeEvent(event)
}

// LogAuthEvent logs authentication events
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

	GlobalEventLogger.writeEvent(event)
}

// LogSystemEvent logs system events
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

// LogError logs error events
func LogError(message, errorMsg, stackTrace string, data map[string]interface{}) {
	event := EventLog{
		ID:         generateEventID(),
		Timestamp:  time.Now(),
		Type:       EventTypeError,
		Level:      LogLevelError,
		Message:    message,
		Error:      errorMsg,
		StackTrace: stackTrace,
		Data:       data,
	}

	GlobalEventLogger.writeEvent(event)
}

// LogDatabaseEvent logs database operations
func LogDatabaseEvent(operation, query string, duration float64, rowsAffected int64, err error) {
	level := LogLevelInfo
	var errorMsg string
	if err != nil {
		level = LogLevelError
		errorMsg = err.Error()
	}

	data := map[string]interface{}{
		"query":         query,
		"rows_affected": rowsAffected,
	}

	event := EventLog{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeDatabase,
		Level:     level,
		Message:   fmt.Sprintf("Database %s", operation),
		Duration:  &duration,
		Error:     errorMsg,
		Data:      data,
	}

	GlobalEventLogger.writeEvent(event)
}

func (el *EventLogger) writeEvent(event EventLog) {
	if event.Source == "" {
		event.Source = capture()
	}
	// Uniform colored console line (same format as the leveled API).
	el.writeToConsole(event)

	// Fan out to subscribers (event emitting).
	emitEvent(el.toEvent(event))

	// Write to file if enabled
	if el.writeToFile && el.logFile != nil {
		el.writeToLogFile(event)
	}

	// Write to database if enabled
	if el.writeToDb {
		el.writeToDatabase(event)
	}
}

// toEvent maps a structured EventLog to the unified Event shape for subscribers.
func (el *EventLogger) toEvent(e EventLog) Event {
	return Event{
		Time:    e.Timestamp,
		Level:   string(e.Level),
		Module:  e.Module,
		Message: e.Message,
		Data:    e.Data,
	}
}

// persist writes a leveled Event (from log.go's LOG_STORE sink) to the configured
// storage sinks only. Console is handled by log.go so it isn't duplicated.
func persist(ev Event) {
	if GlobalEventLogger == nil {
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

// writeToConsole prints one uniform colored line (shared format), tagging the
// module when present, otherwise the event type.
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

// writeToDatabase inserts the event into the `logs` table (schema owned by the
// logs module, go/engine/modules/logs) using the DB injected via Init. The
// column set must match the logs module fieldset.
func (el *EventLogger) writeToDatabase(event EventLog) {
	if db == nil {
		return // no database injected (Init not called, or db logging off)
	}
	row := map[string]interface{}{
		"event_id":    event.ID,
		"ts":          event.Timestamp,
		"type":        string(event.Type),
		"level":       string(dbLevel(event.Level)),
		"message":     event.Message,
		"module":      nullIfEmpty(event.Module),
		"action":      nullIfEmpty(event.Action),
		"session_id":  nullIfEmpty(event.SessionID),
		"request_url": nullIfEmpty(event.RequestURL),
		"method":      nullIfEmpty(event.Method),
		"ip_address":  nullIfEmpty(event.IPAddress),
		"error":       nullIfEmpty(event.Error),
		"source":      nullIfEmpty(event.Source),
	}
	if event.UserID != nil {
		row["user_id"] = *event.UserID
	}
	if event.StatusCode != nil {
		row["status_code"] = *event.StatusCode
	}
	if event.Duration != nil {
		row["duration_ms"] = *event.Duration
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

func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

// EnableDatabaseLogging enables writing events to database
func EnableDatabaseLogging(enable bool) {
	GlobalEventLogger.writeToDb = enable
}

// EnableFileLogging enables writing events to file
func EnableFileLogging(enable bool) {
	GlobalEventLogger.writeToFile = enable
	if enable && GlobalEventLogger.logFile == nil {
		GlobalEventLogger.initLogFile()
	}
}
