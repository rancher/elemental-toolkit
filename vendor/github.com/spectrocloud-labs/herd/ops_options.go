package herd

import "context"

// OpOption defines the operation settings.
type OpOption func(string, *OpState, *Graph) error

var NoOp OpOption = func(s string, os *OpState, g *Graph) error { return nil }

// FatalOp makes the operation fatal.
// Any error will make the DAG to stop and return the error immediately.
var FatalOp OpOption = func(key string, os *OpState, g *Graph) error {
	os.fatal = true
	return nil
}

// Background runs the operation in the background.
var Background OpOption = func(key string, os *OpState, g *Graph) error {
	os.background = true
	return nil
}

// WeakDeps sets all the dependencies of the job as "weak".
// Any failure of the jobs which depends on won't impact running the job.
// By default, a failure job will make also fail all the children - this is option
// disables this behavor and make the child start too.
var WeakDeps OpOption = func(key string, os *OpState, g *Graph) error {
	os.weak = true
	return nil
}

// WithWeakDeps defines dependencies that doesn't prevent the op to trigger.
func WithWeakDeps(deps ...string) OpOption {
	return func(key string, os *OpState, g *Graph) error {

		err := WithDeps(deps...)(key, os, g)
		if err != nil {
			return err
		}
		os.weakdeps = append(os.weakdeps, deps...)
		return nil
	}
}

// WithDeps defines an operation dependency.
// Dependencies can be expressed as a string.
// Note: before running the DAG you must define all the operations.
func WithDeps(deps ...string) OpOption {
	return func(key string, os *OpState, g *Graph) error {
		os.deps = append(os.deps, deps...)

		for _, d := range deps {
			if err := g.Graph.DependOn(key, d); err != nil {
				return err
			}
		}
		return nil
	}
}

// ConditionalOption defines an option that is enabled only if the
// conditional callback returns true.
func ConditionalOption(condition func() bool, op OpOption) OpOption {
	if condition() {
		return op
	}

	return NoOp
}

// IfElse defines options that are enabled if the condition passess or not
// It is just syntax sugar.
func IfElse(condition bool, op, noOp OpOption) OpOption {
	if condition {
		return op
	}

	return noOp
}

// EnableIf defines an operation dependency.
// Dependencies can be expressed as a string.
// Note: before running the DAG you must define all the operations.
func EnableIf(conditional func() bool) OpOption {
	return func(key string, os *OpState, g *Graph) error {
		if !conditional() {
			os.ignore = true
		}
		return nil
	}
}

// WithCallback associates a callback to the operation to be executed
// when the DAG is walked-by.
func WithCallback(fn ...func(context.Context) error) OpOption {
	return func(s string, os *OpState, g *Graph) error {
		os.fn = append(os.fn, fn...)
		return nil
	}
}
