/*
Copyright © 2022 SUSE LLC

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
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/mudler/luet/pkg/api/core/types"
	log "github.com/sirupsen/logrus"
)

// Logger is the interface we want for our logger, so we can plug different ones easily
type Logger interface {
	Info(...interface{})
	Warn(...interface{})
	Debug(...interface{})
	Error(...interface{})
	Fatal(...interface{})
	Success(...interface{})
	Warning(...interface{})
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

	Copy() (types.Logger, error)
	SetContext(string)
	SpinnerStop()
	Spinner()
	Ask() bool
	Screen(string)
}

func DebugLevel() log.Level {
	l, _ := log.ParseLevel("debug")
	return l
}

func IsDebugLevel(l Logger) bool {
	return l.GetLevel() == DebugLevel()
}

type LoggerOptions func(l Logger) error

func NewLogger() Logger {
	return newLogrusWrapper(log.New())
}

// NewNullLogger will return a logger that discards all logs, used mainly for testing
func NewNullLogger() Logger {
	logger := log.New()
	logger.SetOutput(ioutil.Discard)
	return newLogrusWrapper(logger)
}

// NewBufferLogger will return a logger that stores all logs in a buffer, used mainly for testing
func NewBufferLogger(b *bytes.Buffer) Logger {
	logger := log.New()
	logger.SetOutput(b)
	return newLogrusWrapper(logger)
}

type logrusWrapper struct {
	*log.Logger
}

func newLogrusWrapper(l *log.Logger) Logger {
	return &logrusWrapper{Logger: l}
}

func (w *logrusWrapper) Ask() bool {
	var input string
	w.Info("Do you want to continue with this operation? [y/N]: ")
	_, err := fmt.Scanln(&input)
	if err != nil {
		return false
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true
	}
	return false
}

func (w *logrusWrapper) Copy() (types.Logger, error) {
	c := *w
	copy := &c
	return copy, nil
}

func (w *logrusWrapper) Screen(t string) {
	w.Infof(">>>%s", t)
}

// no-ops
func (w *logrusWrapper) Success(r ...interface{}) {
	w.Info(r...)
}
func (w *logrusWrapper) SetContext(string) {}
func (w *logrusWrapper) Spinner()          {}
func (w *logrusWrapper) SpinnerStop()      {}
