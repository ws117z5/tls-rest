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
	// Write to console
	el.writeToConsole(event)

	// Write to file if enabled
	if el.writeToFile && el.logFile != nil {
		el.writeToLogFile(event)
	}

	// Write to database if enabled
	if el.writeToDb {
		el.writeToDatabase(event)
	}
}

func (el *EventLogger) writeToConsole(event EventLog) {
	jsonBytes, _ := json.Marshal(event)
	fmt.Printf("[%s] %s: %s\n", event.Level, event.Type, string(jsonBytes))
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

func (el *EventLogger) writeToDatabase(event EventLog) {
	// TODO: Implement database writing logic
	// This could insert into an events table when db logging is enabled
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
