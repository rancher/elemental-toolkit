/*
Copyright © 2021 SUSE LLC

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

package v1

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
)

// Logger is the interface we want for our logger, so we can plug different ones easily
type Logger interface {
	Info(...interface{})
	Warn(...interface{})
	Debug(...interface{})
	Error(...interface{})
	Fatal(...interface{})
	Panic(...interface{})
	Trace(...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Debugf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})
	Panicf(string, ...interface{})
	Tracef(string, ...interface{})
	SetLevel(level log.Level)
	GetLevel() log.Level
	SetOutput(writer io.Writer)
	SetFormatter(formatter log.Formatter)
}

func DebugLevel() log.Level {
	l, _ := log.ParseLevel("debug")
	return l
}

func IsDebugLevel(l Logger) bool {
	if l.GetLevel() == DebugLevel() {
		return true
	}
	return false
}

type LoggerOptions func(l Logger) error

func NewLogger() Logger {
	return log.New()
}

// NewNullLogger will return a logger that discards all logs, used mainly for testing
func NewNullLogger() Logger {
	logger := log.New()
	logger.SetOutput(ioutil.Discard)
	return logger
}

// NewBufferLogger will return a logger that stores all logs in a buffer, used mainly for testing
func NewBufferLogger(b *bytes.Buffer) Logger {
	logger := log.New()
	logger.SetOutput(b)
	return logger
}
