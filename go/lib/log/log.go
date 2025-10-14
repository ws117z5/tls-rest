package log

import (
	"fmt"

	"github.com/fatih/color"
)

const (
	LOG_PRINT = 1
	LOG_STORE = 3
)

//todo add a logger to store logs in a db

// Logger is a simple logger that uses colors to differentiate between log levels
var DebugLevel = LOG_PRINT

func SetDebugLevel(level int) {
	DebugLevel = level
}

func Info(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		color.Green(message)
	}
}

func Infof(message string, params ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		message = fmt.Sprintf(message, params...)
		color.Green(message)
	}
}

func Warning(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		color.Yellow(message)
	}
}

func Warningf(message string, params ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		message = fmt.Sprintf(message, params...)
		color.Yellow(message)
	}
}

func Error(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		color.Red(message)
	}
}

func Errorf(message string, params ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		message = fmt.Sprintf(message, params...)
		color.Red(message)
	}
}

func Debug(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		color.Cyan(message)
	}
}

func Debugf(message string, params ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		message = fmt.Sprintf(message, params...)
		color.Cyan(message)
	}
}

func Fatal(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		color.Red(message)
	}
	panic(message)
}

func Fatalf(message string, params ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		message = fmt.Sprintf(message, params...)
		color.Red(message)
	}
	panic(message)
}

func Print(message string) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		fmt.Println(message)
	}
}

func Printf(format string, args ...interface{}) {
	if DebugLevel&LOG_PRINT == LOG_PRINT {
		fmt.Printf(format, args...)
	}
}
