// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
)

type levelFlag struct{}

// String implements flag.Value.
func (f levelFlag) String() string {
	return origLogger.Level.String()
}

// Set implements flag.Value.
func (f levelFlag) Set(level string) error {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	origLogger.Level = l
	return nil
}

func init() {
	// In order for this flag to take effect, the user of the package must call
	// flag.Parse() before logging anything.
	flag.Var(levelFlag{}, "log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal].")
}

type Logger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})

	Info(...interface{})
	Infof(string, ...interface{})

	Warn(...interface{})
	Warnf(string, ...interface{})

	Error(...interface{})
	Errorf(string, ...interface{})

	Fatal(...interface{})
	Fatalf(string, ...interface{})

	With(key string, value interface{}) Logger
}

type logger struct {
	entry *logrus.Entry
}

func (l logger) With(key string, value interface{}) Logger {
	return logger{l.entry.WithField(key, value)}
}

// Debug logs a message at level Debug on the standard logger.
func (l logger) Debug(args ...interface{}) {
	l.sourced().Debug(args...)
}

// Debugf logs a message at level Debug on the standard logger.
func (l logger) Debugf(format string, args ...interface{}) {
	l.sourced().Debugf(format, args...)
}

// Info logs a message at level Info on the standard logger.
func (l logger) Info(args ...interface{}) {
	l.sourced().Info(args...)
}

// Infof logs a message at level Info on the standard logger.
func (l logger) Infof(format string, args ...interface{}) {
	l.sourced().Infof(format, args...)
}

// Warn logs a message at level Warn on the standard logger.
func (l logger) Warn(args ...interface{}) {
	l.sourced().Warn(args...)
}

// Warnf logs a message at level Warn on the standard logger.
func (l logger) Warnf(format string, args ...interface{}) {
	l.sourced().Warnf(format, args...)
}

// Error logs a message at level Error on the standard logger.
func (l logger) Error(args ...interface{}) {
	l.sourced().Error(args...)
}

// Errorf logs a message at level Error on the standard logger.
func (l logger) Errorf(format string, args ...interface{}) {
	l.sourced().Errorf(format, args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func (l logger) Fatal(args ...interface{}) {
	l.sourced().Fatal(args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func (l logger) Fatalf(format string, args ...interface{}) {
	l.sourced().Fatalf(format, args...)
}

// sourced adds a source field to the logger that contains
// the file name and line where the logging happened.
func (l logger) sourced() *logrus.Entry {
	fmt.Println("sourced")
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<???>"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		file = file[slash+1:]
	}
	return l.entry.WithField("source", fmt.Sprintf("%s:%d", file, line))
}

var origLogger = logrus.New()
var baseLogger = logger{entry: logrus.NewEntry(origLogger)}

func Base() Logger {
	return baseLogger
}

func With(key string, value interface{}) Logger {
	return baseLogger.With(key, value)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	baseLogger.Debug(args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	baseLogger.Debugf(format, args...)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	baseLogger.Info(args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	baseLogger.Infof(format, args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	baseLogger.Warn(args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	baseLogger.Warnf(format, args...)
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	baseLogger.Error(args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	baseLogger.Errorf(format, args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func Fatal(args ...interface{}) {
	baseLogger.Fatal(args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func Fatalf(format string, args ...interface{}) {
	baseLogger.Fatalf(format, args...)
}
