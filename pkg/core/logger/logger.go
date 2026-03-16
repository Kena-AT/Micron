package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Logger struct {
	level  LogLevel
	logger *log.Logger
}

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(INFO)
}

func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
	}
}

func GetLogger() *Logger {
	return defaultLogger
}

func SetLevelFromString(level string) {
	switch strings.ToLower(level) {
	case "debug":
		defaultLogger.level = DEBUG
	case "info":
		defaultLogger.level = INFO
	case "warn", "warning":
		defaultLogger.level = WARN
	case "error":
		defaultLogger.level = ERROR
	default:
		defaultLogger.level = INFO
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.logger.SetPrefix("[DEBUG] ")
		l.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.logger.SetPrefix("[INFO] ")
		l.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.logger.SetPrefix("[WARN] ")
		l.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.logger.SetPrefix("[ERROR] ")
		l.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func Debug(format string, v ...interface{}) {
	defaultLogger.Debug(format, v...)
}

func Info(format string, v ...interface{}) {
	defaultLogger.Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	defaultLogger.Warn(format, v...)
}

func Error(format string, v ...interface{}) {
	defaultLogger.Error(format, v...)
}
