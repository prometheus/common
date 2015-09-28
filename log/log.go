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
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
)

var logger = logrus.New()

type levelFlag struct{}

// String implements flag.Value.
func (f levelFlag) String() string {
	return logger.Level.String()
}

// Set implements flag.Value.
func (f levelFlag) Set(level string) error {
	l, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logger.Level = l
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

	With(keyvals ...interface{}) Logger
}

func With(key string) Logger {
	return logger.WithField(key, value)
}

// fileLineEntry returns a logrus.Entry with file and line annotations for the
// original user log statement (two stack frames up from this function).
func fileLineEntry() *logrus.Entry {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<???>"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		file = file[slash+1:]
	}
	return logger.WithFields(logrus.Fields{
		"source": file + ":" + line,
	})
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	fileLineEntry().Debug(args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	fileLineEntry().Debugf(format, args...)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	fileLineEntry().Info(args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	fileLineEntry().Infof(format, args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	fileLineEntry().Warn(args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	fileLineEntry().Warnf(format, args...)
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	fileLineEntry().Error(args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	fileLineEntry().Errorf(format, args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func Fatal(args ...interface{}) {
	fileLineEntry().Fatal(args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func Fatalf(format string, args ...interface{}) {
	fileLineEntry().Fatalf(format, args...)
}
