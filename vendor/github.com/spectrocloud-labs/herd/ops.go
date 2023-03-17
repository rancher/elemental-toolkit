package herd

import (
	"context"
	"sync"
)

type OpState struct {
	sync.Mutex
	fn         []func(context.Context) error
	err        error
	fatal      bool
	background bool
	executed   bool
	weak       bool
	weakdeps   []string
	deps       []string
	ignore     bool
}

func (o *OpState) toGraphEntry(name string) GraphEntry {
	return GraphEntry{
		WithCallback:     o.fn != nil,
		Callback:         o.fn,
		Error:            o.err,
		Executed:         o.executed,
		Background:       o.background,
		WeakDeps:         o.weak,
		Dependencies:     o.deps,
		WeakDependencies: o.weakdeps,
		Fatal:            o.fatal,
		Name:             name,
		Ignored:          o.ignore,
	}
}
