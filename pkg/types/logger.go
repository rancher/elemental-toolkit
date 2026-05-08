/*
Copyright © 2022 - 2026 SUSE LLC

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

package types

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

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
	logger.SetOutput(io.Discard)
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

func (w *logrusWrapper) Success(r ...interface{}) {
	// Will redirect to the Info method below and be cleaned there
	w.Info(r...)
}

var emojiStrip = regexp.MustCompile(`[:][\w]+[:]`)

func (w *logrusWrapper) Debug(args ...interface{}) {
	converted := convert(args)
	w.Logger.Debug(converted)
}

func (w *logrusWrapper) Info(args ...interface{}) {
	converted := convert(args)
	w.Logger.Info(converted)
}

func (w *logrusWrapper) Warn(args ...interface{}) {
	converted := convert(args)
	w.Logger.Warn(converted)
}

func (w *logrusWrapper) Error(args ...interface{}) {
	converted := convert(args)
	w.Logger.Error(converted)
}

func (w *logrusWrapper) Fatal(args ...interface{}) {
	converted := convert(args)
	w.Logger.Fatal(converted)
}

// convert changes a list of interfaces into a proper joined string ready to log
func convert(args []interface{}) string {
	var together []string
	// Matches a :WORD: and any extra space after that and the next word to remove emojis
	// which are like ":house: realMessageStartsHere"
	emojiStrip = regexp.MustCompile(`[:][\w]+[:]\s`)
	for _, a := range args {
		toClean := fmt.Sprintf("%v", a)                     // coerce into string
		cleaned := emojiStrip.ReplaceAllString(toClean, "") // remove any emoji
		trimmed := strings.Trim(cleaned, " ")               // trim any spaces in prefix/suffix
		together = append(together, trimmed)
	}
	return strings.Join(together, " ") // return them nicely joined with spaces like a normal phrase
}

func (w *logrusWrapper) SetContext(string) {}
func (w *logrusWrapper) Spinner()          {}
func (w *logrusWrapper) SpinnerStop()      {}
func (w *logrusWrapper) Screen(_ string)   {}
