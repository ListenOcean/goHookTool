package log

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level = zapcore.Level

const (
	InfoLevel   = zap.InfoLevel   // 0, default level
	WarnLevel   = zap.WarnLevel   // 1
	ErrorLevel  = zap.ErrorLevel  // 2
	DPanicLevel = zap.DPanicLevel // 3, used in development log
	// PanicLevel logs a message, then panics
	PanicLevel = zap.PanicLevel // 4
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel = zap.FatalLevel // 5
	DebugLevel = zap.DebugLevel // -1
)

type Field = zap.Field

// function variables for all field types
// in github.com/uber-go/zap/field.go

var (
	Skip        = zap.Skip
	Binary      = zap.Binary
	Bool        = zap.Bool
	Boolp       = zap.Boolp
	ByteString  = zap.ByteString
	Complex128  = zap.Complex128
	Complex128p = zap.Complex128p
	Complex64   = zap.Complex64
	Complex64p  = zap.Complex64p
	Float64     = zap.Float64
	Float64p    = zap.Float64p
	Float32     = zap.Float32
	Float32p    = zap.Float32p
	Int         = zap.Int
	Intp        = zap.Intp
	Int64       = zap.Int64
	Int64p      = zap.Int64p
	Int32       = zap.Int32
	Int32p      = zap.Int32p
	Int16       = zap.Int16
	Int16p      = zap.Int16p
	Int8        = zap.Int8
	Int8p       = zap.Int8p
	String      = zap.String
	Stringp     = zap.Stringp
	Uint        = zap.Uint
	Uintp       = zap.Uintp
	Uint64      = zap.Uint64
	Uint64p     = zap.Uint64p
	Uint32      = zap.Uint32
	Uint32p     = zap.Uint32p
	Uint16      = zap.Uint16
	Uint16p     = zap.Uint16p
	Uint8       = zap.Uint8
	Uint8p      = zap.Uint8p
	Uintptr     = zap.Uintptr
	Uintptrp    = zap.Uintptrp
	Reflect     = zap.Reflect
	Namespace   = zap.Namespace
	Stringer    = zap.Stringer
	Time        = zap.Time
	Timep       = zap.Timep
	Stack       = zap.Stack
	StackSkip   = zap.StackSkip
	Duration    = zap.Duration
	Durationp   = zap.Durationp
	Any         = zap.Any

	Info = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Info(msg, fields...)
		}
	}
	Warn = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Warn(msg, fields...)
		}
	}
	Error = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Error(msg, fields...)
		}
	}
	DPanic = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.DPanic(msg, fields...)
		}
	}
	Panic = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Panic(msg, fields...)
		}
	}
	Fatal = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Fatal(msg, fields...)
		}
	}
	Debug = func(msg string, fields ...zap.Field) {
		if stdLogger != nil {
			stdLogger.Debug(msg, fields...)
		}
	}
)

type Logger struct {
	*zap.Logger // zap ensure that zap.Logger is safe for concurrent use
	level       Level
}

func Default() *Logger {
	return stdLogger
}

var stdLogger *Logger

var LogFileName string

func Clear() {
	Sync()
	if LogFileName != "" {
		_ = os.RemoveAll(LogFileName)
	}
}

// New create a new logger (not support log rotating).
func New(logType string) *Logger {
	debugEncoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "ts",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "func",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if logType == "Debug" {
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(debugEncoderCfg),
			zapcore.AddSync(os.Stdout),
			DebugLevel,
		)
		logger := &Logger{
			Logger: zap.New(consoleCore, zap.AddCaller()),
			level:  DebugLevel}
		return logger
	} else {
		LogFileName = os.TempDir() + fmt.Sprintf("/.autobuild_go_%s.log", time.Now().Format(time.RFC3339))
		logFile, err := os.OpenFile(LogFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			logFile = nil
		}
		fileEncoderCfg := zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			FunctionKey:    "func",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   nil,
		}
		fileCore := zapcore.NewCore(zapcore.NewConsoleEncoder(fileEncoderCfg), zapcore.AddSync(logFile), DebugLevel)

		consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(debugEncoderCfg), zapcore.AddSync(os.Stdout), InfoLevel)

		logger := &Logger{
			Logger: zap.New(zapcore.NewTee(consoleCore, fileCore)),
			level:  DebugLevel}
		return logger
	}
}

func Sync() error {
	if stdLogger != nil {
		return stdLogger.Sync()
	}
	return nil
}

func CheckAndCleanLogFile(filename string) {
	checkInterval := time.Hour * 24 * 3
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return
	}

	if time.Since(fileInfo.ModTime()) > checkInterval {
		err = os.Truncate(filename, 0)
		if err != nil {
			return
		}
	}
}

func InitLog() {
	stdLogger = New("Release")
}
