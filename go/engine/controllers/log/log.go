package log

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"tls-rest/go/constants"

	"github.com/fatih/color"
)

func init() {
	color.NoColor = false
}

// Output sinks, combined as a bitmask.
//
//	LOG_PRINT — colored console lines (uniform across the whole app).
//	LOG_STORE — structured persistence to file/db (handled in events.go).
const (
	LOG_PRINT = 1 << iota // 1
	LOG_STORE             // 2
)

// DebugLevel is the default sink mask used by the package-level functions and by
// the Default logger. Console-only out of the box; call Init to add storage.
var DebugLevel = LOG_PRINT

// SetDebugLevel replaces the active default sink mask.
func SetDebugLevel(level int) { DebugLevel = level }

// level bundles a severity's name with its console color.
type level struct {
	name  string
	color *color.Color
}

var (
	levelDebug   = level{"debug", color.New(color.FgHiBlack)}
	levelInfo    = level{"info", color.New(color.FgCyan)}
	levelSuccess = level{"success", color.New(color.FgGreen, color.Bold)}
	levelWarn    = level{"warn", color.New(color.FgYellow)}
	levelError   = level{"error", color.New(color.FgRed)}
	levelFatal   = level{"fatal", color.New(color.FgRed, color.Bold)}
)

func levelByName(name string) level {
	switch name {
	case "debug":
		return levelDebug
	case "success":
		return levelSuccess
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	case "fatal":
		return levelFatal
	default:
		return levelInfo
	}
}

// printLine writes one uniform colored console line. Shared by the leveled API
// here and the structured event logger in events.go, so console output looks the
// same everywhere. source ("pkg/file.go:line") and module are optional.
func printLine(l level, module, source, msg string) {
	prefix := source
	if module != "" {
		if prefix != "" {
			prefix = module + " " + prefix
		} else {
			prefix = module
		}
	}
	if prefix != "" {
		l.color.Printf("[%s] %s: %s\n", l.name, prefix, msg)
	} else {
		l.color.Printf("[%s] %s\n", l.name, msg)
	}
}

// capture returns "pkg/file.go:line" of the first caller outside this package, so
// leveled and structured logs both report where they were emitted — regardless
// of how many internal wrapper frames sit in between.
func capture() string {
	pcs := make([]uintptr, 24)
	n := runtime.Callers(2, pcs) // skip runtime.Callers + capture
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		fr, more := frames.Next()
		if fr.File != "" && !strings.Contains(fr.File, "/engine/controllers/log/") {
			return shortFile(fr.File) + ":" + strconv.Itoa(fr.Line)
		}
		if !more {
			break
		}
	}
	return ""
}

// shortFile trims an absolute path to its last two elements: "pkg/file.go".
func shortFile(f string) string {
	i := strings.LastIndexByte(f, '/')
	if i < 0 {
		return f
	}
	if j := strings.LastIndexByte(f[:i], '/'); j >= 0 {
		return f[j+1:]
	}
	return f[i+1:]
}

func sf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Event is one structured log record, delivered to subscribers and (optionally)
// to storage. It is the single shape every log call funnels into.
type Event struct {
	Time    time.Time
	Level   string
	Module  string
	Message string
	Source  string // "pkg/file.go:line" where the log call was made
	Data    map[string]interface{}
}

// --- event emitting -------------------------------------------------------

var (
	subMu       sync.RWMutex
	subscribers = map[int]func(Event){}
	subSeq      int
)

// Subscribe registers a listener that receives every emitted event. The returned
// function unsubscribes. Use it to fan events out to metrics, alerting, SSE, etc.
func Subscribe(fn func(Event)) (cancel func()) {
	subMu.Lock()
	subSeq++
	id := subSeq
	subscribers[id] = fn
	subMu.Unlock()
	return func() {
		subMu.Lock()
		delete(subscribers, id)
		subMu.Unlock()
	}
}

func emitEvent(ev Event) {
	subMu.RLock()
	for _, fn := range subscribers {
		fn(ev)
	}
	subMu.RUnlock()
}

// --- Logger (destination control) -----------------------------------------

// Logger writes to a chosen set of sinks and optionally tags a module. Use
// Console for console-only output that never touches file/db, Default for the
// configured sinks, or For(module)/With(module) to tag lines.
type Logger struct {
	Sinks  int    // 0 => follow the global DebugLevel
	Module string // optional tag shown as "[level] module: msg"
}

var (
	// Default follows DebugLevel (console, plus storage once Init enables it).
	Default = &Logger{}
	// Console always prints to the console only — never file/db.
	Console = &Logger{Sinks: LOG_PRINT}
)

// For returns a Default-sinked logger tagged with a module name.
func For(module string) *Logger { return &Logger{Module: module} }

// With returns a copy of the logger tagged with a module name.
func (lg *Logger) With(module string) *Logger { c := *lg; c.Module = module; return &c }

// To returns a copy of the logger writing to exactly the given sinks.
func (lg *Logger) To(sinks int) *Logger { c := *lg; c.Sinks = sinks; return &c }

func (lg *Logger) activeSinks() int {
	if lg.Sinks != 0 {
		return lg.Sinks
	}
	return DebugLevel
}

// emit is the single funnel: colored console (if enabled) → subscribers → store.
func (lg *Logger) emit(l level, msg string) {
	s := lg.activeSinks()
	source := capture()
	if s&LOG_PRINT == LOG_PRINT {
		printLine(l, lg.Module, source, msg)
	}
	ev := Event{Time: time.Now(), Level: l.name, Module: lg.Module, Message: msg, Source: source}
	emitEvent(ev)
	if s&LOG_STORE == LOG_STORE {
		persist(ev) // events.go: file/db only (no console — already printed)
	}
}

func (lg *Logger) Debug(m string)                      { lg.emit(levelDebug, m) }
func (lg *Logger) Debugf(f string, a ...interface{})   { lg.emit(levelDebug, sf(f, a...)) }
func (lg *Logger) Info(m string)                       { lg.emit(levelInfo, m) }
func (lg *Logger) Infof(f string, a ...interface{})    { lg.emit(levelInfo, sf(f, a...)) }
func (lg *Logger) Success(m string)                    { lg.emit(levelSuccess, m) }
func (lg *Logger) Successf(f string, a ...interface{}) { lg.emit(levelSuccess, sf(f, a...)) }
func (lg *Logger) Warn(m string)                       { lg.emit(levelWarn, m) }
func (lg *Logger) Warnf(f string, a ...interface{})    { lg.emit(levelWarn, sf(f, a...)) }
func (lg *Logger) Error(m string)                      { lg.emit(levelError, m) }
func (lg *Logger) Errorf(f string, a ...interface{})   { lg.emit(levelError, sf(f, a...)) }

// --- package-level convenience (delegate to Default) ----------------------

func Debug(m string)                      { Default.Debug(m) }
func Debugf(f string, a ...interface{})   { Default.Debugf(f, a...) }
func Info(m string)                       { Default.Info(m) }
func Infof(f string, a ...interface{})    { Default.Infof(f, a...) }
func Success(m string)                    { Default.Success(m) }
func Successf(f string, a ...interface{}) { Default.Successf(f, a...) }
func Warn(m string)                       { Default.Warn(m) }
func Warnf(f string, a ...interface{})    { Default.Warnf(f, a...) }
func Error(m string)                      { Default.Error(m) }
func Errorf(f string, a ...interface{})   { Default.Errorf(f, a...) }

// Warning / Warningf are retained aliases.
func Warning(m string)                    { Default.Warn(m) }
func Warningf(f string, a ...interface{}) { Default.Warnf(f, a...) }

// Fatal logs at fatal level then panics.
func Fatal(m string) {
	source := capture()
	printLine(levelFatal, "", source, m)
	emitEvent(Event{Time: time.Now(), Level: "fatal", Message: m, Source: source})
	panic(m)
}
func Fatalf(f string, a ...interface{}) { Fatal(sf(f, a...)) }

// Print / Printf / Println write uncolored lines, gated by the console sink.
// Prefer the leveled functions; these remain for incidental output.
func Print(msg string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		fmt.Println(msg)
	}
}
func Printf(format string, args ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		fmt.Printf(format, args...)
	}
}
func Println(args ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		fmt.Println(args...)
	}
}

// --- storage wiring -------------------------------------------------------

// dbLevel maps console levels to the values allowed by the logs table's
// log_level enum (debug|info|warn|error). success -> info, fatal -> error, so a
// colored console level never breaks the database write.
func dbLevel(l LogLevel) LogLevel {
	switch l {
	case "debug", "info", "warn", "error":
		return l
	case "success":
		return "info"
	case "warning":
		return "warn"
	case "fatal":
		return "error"
	default:
		return "info"
	}
}

// DB is the minimal database surface the event logger needs. Injected via Init;
// the log package never imports the db layer (pgdb imports this package).
type DB interface {
	InsertRow(table string, row map[string]interface{}) (int64, error)
}

var db DB

// Init is the single logging entry point. Call once from main after config is
// loaded, passing the database to log into. It stores the db and applies the
// configured file/db sinks from go.config.json "log".
func Init(database DB) {
	db = database
	EnableFileLogging(constants.Config.Log.WriteToFile)
	EnableDatabaseLogging(constants.Config.Log.WriteToDb)
	if constants.Config.Log.WriteToFile || constants.Config.Log.WriteToDb {
		DebugLevel |= LOG_STORE
	}
}
