package log

import (
	"fmt"

	"tls-rest/go/constants"

	"github.com/fatih/color"
)

// Output sinks, combined as a bitmask in DebugLevel.
//
//	LOG_PRINT — colored lines to the console (this file).
//	LOG_STORE — structured persistence to file/db, handled by the event logger
//	            in events.go (see LogSystemEvent / GlobalEventLogger). log.go
//	            itself only ever prints; storage is deliberately kept there so a
//	            noisy console call can't accidentally hit the database.
const (
	LOG_PRINT = 1 << iota // 1
	LOG_STORE             // 2
)

// DebugLevel controls which sinks are active. Defaults to console only.
var DebugLevel = LOG_PRINT

// SetDebugLevel replaces the active sink bitmask.
func SetDebugLevel(level int) { DebugLevel = level }

// level bundles a severity's console color with its name, so adding a severity
// is one line below rather than a new pair of copy-pasted functions.
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

// emit prints "[level] msg" in the level's color when the console sink is enabled.
func (l level) emit(msg string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		l.color.Printf("[%s] %s\n", l.name, msg)
	}
}

func sf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Debug — dim/high-black.
func Debug(msg string)                  { levelDebug.emit(msg) }
func Debugf(f string, a ...interface{}) { levelDebug.emit(sf(f, a...)) }

// Info — cyan.
func Info(msg string)                  { levelInfo.emit(msg) }
func Infof(f string, a ...interface{}) { levelInfo.emit(sf(f, a...)) }

// Success — bold green.
func Success(msg string)                  { levelSuccess.emit(msg) }
func Successf(f string, a ...interface{}) { levelSuccess.emit(sf(f, a...)) }

// Warn — yellow.
func Warn(msg string)                  { levelWarn.emit(msg) }
func Warnf(f string, a ...interface{}) { levelWarn.emit(sf(f, a...)) }

// Error — red.
func Error(msg string)                  { levelError.emit(msg) }
func Errorf(f string, a ...interface{}) { levelError.emit(sf(f, a...)) }

// Fatal — bold red, then panic.
func Fatal(msg string) {
	levelFatal.emit(msg)
	panic(msg)
}
func Fatalf(f string, a ...interface{}) {
	msg := sf(f, a...)
	levelFatal.emit(msg)
	panic(msg)
}

// Warning / Warningf are retained aliases for the previous API.
func Warning(msg string)                  { Warn(msg) }
func Warningf(f string, a ...interface{}) { Warnf(f, a...) }

// Print / Printf write uncolored lines, still gated by the console sink.
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

// DB is the minimal database surface the event logger needs to persist events.
// It is an interface the log package owns, injected via Init — the log package
// never imports the db layer. That's deliberate: pgdb imports this package (for
// its own diagnostics), so importing pgdb here would be an import cycle.
// *pgdb.Db satisfies this interface, so main passes it straight in.
type DB interface {
	InsertRow(table string, row map[string]interface{}) (int64, error)
}

// db is the injected database used when db logging is enabled (nil until Init).
var db DB

// Init is the single logging entry point. Call once from main, after config is
// loaded, passing the database to log into:
//
//	database, _ := pgdb.GetInstance()
//	log.Init(database)
//
// It stores the db and applies the configured sinks (file/db) from
// go.config.json "log". All logging wiring lives here.
func Init(database DB) {
	db = database
	EnableFileLogging(constants.Config.Log.WriteToFile)
	EnableDatabaseLogging(constants.Config.Log.WriteToDb)
}
