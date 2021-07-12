/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clog

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitCubeLoggerWithOpts init cube logger with options
func InitCubeLoggerWithOpts(opts *Config) {
	zapLogger := zap.New(newZapCore(opts), zap.AddCaller(), zap.AddCallerSkip(2),
		zap.Development(), zap.AddStacktrace(levelAdapt(opts.StacktraceLevel)))

	logger = &cubeLogger{l: zapLogger}
}

func newDefaultCubeLogger() CubeLogger {
	return New(&Config{
		LogFile:         "",
		MaxAge:          0,
		MaxBackups:      0,
		MaxSize:         0,
		Compress:        false,
		LogLevel:        "info",
		JsonEncode:      false,
		StacktraceLevel: "error",
	})
}

// New create cube logger with options
func New(opts *Config) CubeLogger {
	zapLogger := zap.New(newZapCore(opts), zap.AddCaller(), zap.AddCallerSkip(2),
		zap.Development(), zap.AddStacktrace(levelAdapt(opts.StacktraceLevel)))

	return &cubeLogger{l: zapLogger}
}

// newZapCore new zap core and hook for zap logger
func newZapCore(opts *Config) zapcore.Core {
	hook := lumberjack.Logger{
		Filename:   opts.LogFile,
		MaxSize:    opts.MaxSize,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAge,
		Compress:   opts.Compress,
	}

	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(levelAdapt(opts.LogLevel))

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "line",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	if opts.JsonEncode {
		zapcore.NewJSONEncoder(encoderConfig)
	}

	writeSyncer := zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&hook))

	// only output to console if log file if empty
	if opts.LogFile == "" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	return zapcore.NewCore(encoder, writeSyncer, atomicLevel)
}

func (c *cubeLogger) AddCallerSkip(callerSkip int) CubeLogger {
	return newCubeLoggerWithExtraSkip(c.l, callerSkip)
}

func (c *cubeLogger) WithName(name string) CubeLogger {
	l := c.l.Named(name)
	return newCubeLoggerWithExtraSkip(l, 0)
}

func (c *cubeLogger) WithValues(keysAndValues ...interface{}) CubeLogger {
	l := c.l.With(handleFields(c.l, keysAndValues)...)
	return newCubeLoggerWithExtraSkip(l, 0)
}

func (c *cubeLogger) Debug(format string, a ...interface{}) {
	c.l.Debug(fmt.Sprintf(format, a...))
}

func (c *cubeLogger) Info(format string, a ...interface{}) {
	c.l.Info(fmt.Sprintf(format, a...))
}

func (c *cubeLogger) Warn(format string, a ...interface{}) {
	c.l.Warn(fmt.Sprintf(format, a...))
}

func (c *cubeLogger) Error(format string, a ...interface{}) {
	c.l.Error(fmt.Sprintf(format, a...))
}

func (c *cubeLogger) Fatal(format string, a ...interface{}) {
	c.l.Fatal(fmt.Sprintf(format, a...))
}

// copy form http://github.com/go-logr/zapr/zapr.go
// handleFields converts a bunch of arbitrary key-value pairs into Zap fields.  It takes
// additional pre-converted Zap fields, for use with automatically attached fields, like
// `error`.
func handleFields(l *zap.Logger, args []interface{}, additional ...zap.Field) []zap.Field {
	// a slightly modified version of zap.SugaredLogger.sweetenFields
	if len(args) == 0 {
		// fast-return if we have no suggared fields.
		return additional
	}

	// unlike Zap, we can be pretty sure users aren't passing structured
	// fields (since logr has no concept of that), so guess that we need a
	// little less space.
	fields := make([]zap.Field, 0, len(args)/2+len(additional))
	for i := 0; i < len(args); {
		// check just in case for strongly-typed Zap fields, which is illegal (since
		// it breaks implementation agnosticism), so we can give a better error message.
		if _, ok := args[i].(zap.Field); ok {
			l.DPanic("strongly-typed Zap Field passed to logr", zap.Any("zap field", args[i]))
			break
		}

		// make sure this isn't a mismatched key
		if i == len(args)-1 {
			l.DPanic("odd number of arguments passed as key-value pairs for logging", zap.Any("ignored key", args[i]))
			break
		}

		// process a key-value pair,
		// ensuring that the key is a string
		key, val := args[i], args[i+1]
		keyStr, isString := key.(string)
		if !isString {
			// if the key isn't a string, DPanic and stop logging
			l.DPanic("non-string key argument passed to logging, ignoring all later arguments", zap.Any("invalid key", key))
			break
		}

		fields = append(fields, zap.Any(keyStr, val))
		i += 2
	}

	return append(fields, additional...)
}

// newCubeLoggerWithExtraSkip allows creation of loggers with variable levels of callstack skipping
func newCubeLoggerWithExtraSkip(l *zap.Logger, callerSkip int) CubeLogger {
	_l := l.WithOptions(zap.AddCallerSkip(callerSkip))
	return &cubeLogger{l: _l}
}

// levelAdapt adapts cube logger level to zap logger level
func levelAdapt(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		log.Fatalf("unknown level %s", level)
	}

	return 0
}
