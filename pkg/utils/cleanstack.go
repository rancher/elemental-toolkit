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

package utils

import (
	"github.com/hashicorp/go-multierror"
)

const (
	errorOnly = iota
	successOnly
	always
)

type CleanFunc func() error

// CleanJob represents a clean task. In can be of three different types. ErrorOnly type
// is a clean job only executed on error, successOnly type is executed only on sucess and always
// is always executed regardless the error value.
type CleanJob struct {
	cleanFunc CleanFunc
	jobType   int
}

// Run executes the defined job
func (cj CleanJob) Run() error {
	return cj.cleanFunc()
}

// Type returns the CleanJob type
func (cj CleanJob) Type() int {
	return cj.jobType
}

// NewCleanStack returns a new stack.
func NewCleanStack() *CleanStack {
	return &CleanStack{}
}

// Stack is a basic LIFO stack that resizes as needed.
type CleanStack struct {
	jobs  []*CleanJob
	count int
}

// Push adds a node to the stack that will be always executed
func (clean *CleanStack) Push(cFunc CleanFunc) {
	clean.jobs = append(clean.jobs[:clean.count], &CleanJob{cleanFunc: cFunc, jobType: always})
	clean.count++
}

// PushErrorOnly adds an error only node to the stack
func (clean *CleanStack) PushErrorOnly(cFunc CleanFunc) {
	clean.jobs = append(clean.jobs[:clean.count], &CleanJob{cleanFunc: cFunc, jobType: errorOnly})
	clean.count++
}

// PushSuccessOnly adds a success only node to the stack
func (clean *CleanStack) PushSuccessOnly(cFunc CleanFunc) {
	clean.jobs = append(clean.jobs[:clean.count], &CleanJob{cleanFunc: cFunc, jobType: successOnly})
	clean.count++
}

// Pop removes and returns a node from the stack in last to first order.
func (clean *CleanStack) Pop() *CleanJob {
	if clean.count == 0 {
		return nil
	}
	clean.count--
	return clean.jobs[clean.count]
}

// Cleanup runs the whole cleanup stack. In case of error it runs all jobs
// and returns the first error occurrence.
func (clean *CleanStack) Cleanup(err error) error {
	var errs error
	if err != nil {
		errs = multierror.Append(errs, err)
	}
	for clean.count > 0 {
		job := clean.Pop()
		switch job.Type() {
		case successOnly:
			if errs == nil {
				errs = runCleanJob(job, errs)
			}
		case errorOnly:
			if errs != nil {
				errs = runCleanJob(job, errs)
			}
		default:
			errs = runCleanJob(job, errs)
		}
	}
	return errs
}

func runCleanJob(job *CleanJob, errs error) error {
	err := job.Run()
	if err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}
