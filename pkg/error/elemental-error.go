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

package error

// ElementalError is our custom error to pass around exit codes in the error
type ElementalError struct {
	err  string
	code int
}

func (e *ElementalError) Error() string {
	return e.err
}

func (e *ElementalError) ExitCode() int {
	return e.code
}

// NewFromError generates an ElementalError from an existing error,
// maintaining its error message
func NewFromError(err error, code int) error {
	if err == nil {
		return nil
	}

	errorMsg := ""
	if err.Error() != "" {
		errorMsg = err.Error()
	}
	return &ElementalError{err: errorMsg, code: code}
}

// New generates an ElementalError from a string
func New(err string, code int) error {
	return &ElementalError{err: err, code: code}
}
