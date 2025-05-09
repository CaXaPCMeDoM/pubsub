package logger

import (
	"log"
	"os"
	"strings"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

var levelNames = map[Level]string{
	DebugLevel: "DEBUG",
	InfoLevel:  "INFO",
	WarnLevel:  "WARN",
	ErrorLevel: "ERROR",
	FatalLevel: "FATAL",
}

var levelValues = map[string]Level{
	"debug": DebugLevel,
	"info":  InfoLevel,
	"warn":  WarnLevel,
	"error": ErrorLevel,
	"fatal": FatalLevel,
}

type Logger struct {
	level  Level
	logger *log.Logger
}

func New(level string) *Logger {
	l, exists := levelValues[strings.ToLower(level)]
	if !exists {
		l = InfoLevel
	}

	return &Logger{
		level:  l,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, format, v...)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, format, v...)
	}
}

func (l *Logger) Warn(format string, v ...interface{}) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, format, v...)
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, format, v...)
	}
}

func (l *Logger) Fatal(format string, v ...interface{}) {
	if l.level <= FatalLevel {
		l.log(FatalLevel, format, v...)
		os.Exit(1)
	}
}

func (l *Logger) log(level Level, format string, v ...interface{}) {
	prefix := "[" + levelNames[level] + "] "
	l.logger.Printf(prefix+format, v...)
}
