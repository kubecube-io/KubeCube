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
	"go.uber.org/zap"
)

type CubeLogger interface {
	// AddCallerSkip new cube logger with callstack skipping.
	AddCallerSkip(callerSkip int) CubeLogger

	// WithName adds some key-value pairs of context to a logger.
	// See Info for documentation on how key/value pairs work.
	WithName(name string) CubeLogger

	// WithValues adds a new element to the logger's name.
	// Successive calls with WithName continue to append
	// suffixes to the logger's name.  It's strongly recommended
	// that name segments contain only letters, digits, and hyphens
	// (see the package documentation for more information).
	WithValues(keysAndValues ...interface{}) CubeLogger

	Debug(format string, a ...interface{})

	Info(format string, a ...interface{})

	Warn(format string, a ...interface{})

	Error(format string, a ...interface{})

	Fatal(format string, a ...interface{})
}

type cubeLogger struct {
	l *zap.Logger
}

var logger CubeLogger

func Debug(format string, a ...interface{}) {
	ensureLogger().Debug(format, a...)
}

func Info(format string, a ...interface{}) {
	ensureLogger().Info(format, a...)
}

func Warn(format string, a ...interface{}) {
	ensureLogger().Warn(format, a...)
}

func Error(format string, a ...interface{}) {
	ensureLogger().Error(format, a...)
}

func Fatal(format string, a ...interface{}) {
	ensureLogger().Fatal(format, a...)
}

func WithName(name string) CubeLogger {
	return ensureLogger().WithName(name).AddCallerSkip(-1)
}

func WithValues(keysAndValues ...interface{}) CubeLogger {
	return ensureLogger().WithValues(keysAndValues).AddCallerSkip(-1)
}

// ensureLogger new default cube logger if logger is nil
func ensureLogger() CubeLogger {
	if logger == nil {
		logger = newDefaultCubeLogger()
	}
	return logger
}
