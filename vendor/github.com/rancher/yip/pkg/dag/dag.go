package dag

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/kendru/darwin/go/depgraph"
)

// Graph represents a directed graph.
type Graph struct {
	*depgraph.Graph
	ops            map[string]*OpState
	init           bool
	orphans        *sync.WaitGroup
	collectOrphans bool
}

// GraphEntry is the external representation of
// the operation to execute (OpState).
type GraphEntry struct {
	WithCallback                       bool
	Background                         bool
	Callback                           []func(context.Context) error
	Error                              error
	Ignored, Fatal, WeakDeps, Executed bool
	Name                               string
	Dependencies                       []string
	WeakDependencies                   []string
	Duration                           time.Duration
}

// DAG creates a new instance of a runnable Graph.
// A DAG is a Direct Acyclic Graph.
// The graph is walked, and depending on the dependencies it will run the jobs as requested.
// The Graph can be explored with `Analyze()`, extended with new operations with Add(),
// and finally being run with Run(context.Context).
func DAG(opts ...GraphOption) *Graph {
	g := &Graph{Graph: depgraph.New(), ops: make(map[string]*OpState), orphans: &sync.WaitGroup{}}
	for _, o := range opts {
		o(g)
	}
	if g.init {
		if err := g.Add("init"); err != nil {
			return nil
		}
	}
	return g
}

// Add adds a new operation to the graph.
// Requires a name (string), and accepts a list of options.
func (g *Graph) Add(name string, opts ...OpOption) error {
	state := &OpState{Mutex: sync.Mutex{}}

	for _, o := range opts {
		if err := o(name, state, g); err != nil {
			return err
		}
	}

	g.ops[name] = state

	if g.init && len(g.Graph.Dependents(name)) == 0 && name != "init" {
		if err := g.Graph.DependOn(name, "init"); err != nil {
			return err
		}
	}

	return nil
}

// Stage returns the DAG item state.
// Note: it locks to be thread-safe.
func (g *Graph) State(name string) GraphEntry {
	g.ops[name].Lock()
	defer g.ops[name].Unlock()
	return g.ops[name].toGraphEntry(name)
}

func (g *Graph) buildStateGraph() (graph [][]GraphEntry) {
	for _, layer := range g.TopoSortedLayers() {
		states := []GraphEntry{}

		for _, r := range layer {
			g.ops[r].Lock()
			states = append(states, g.ops[r].toGraphEntry(r))
			g.ops[r].Unlock()
		}

		graph = append(graph, states)
	}
	return
}

// Analyze returns the DAG and the Graph in the execution order.
// It will also return eventual updates if called after Run().
func (g *Graph) Analyze() (graph [][]GraphEntry) {
	return g.buildStateGraph()
}

// Run starts the jobs defined in the DAG with a context.
// It returns error in case of failure.
func (g *Graph) Run(ctx context.Context) error {

	checkFatal := func(layer []GraphEntry) error {
		for _, s := range layer {
			if s.Fatal && g.ops[s.Name].err != nil {
				return g.ops[s.Name].err
			}
		}
		return nil
	}

	for _, layer := range g.buildStateGraph() {
		var wg sync.WaitGroup

	LAYER:
		for _, r := range layer {
			if !r.WithCallback || r.Ignored {
				continue
			}
			fns := r.Callback

			if !r.WeakDeps {
				for k := range g.Graph.Dependencies(r.Name) {
					if len(r.WeakDependencies) != 0 && slices.Contains(r.WeakDependencies, k) {
						continue
					}

					g.ops[r.Name].Lock()
					g.ops[k].Lock()

					unlock := func() {
						g.ops[r.Name].Unlock()
						g.ops[k].Unlock()
					}

					if g.ops[k].err != nil {
						g.ops[r.Name].err = fmt.Errorf("'%s' deps %s failed", r.Name, k)
						unlock()

						continue LAYER
					}
					unlock()
				}
			}

			for i := range fns {
				if !r.Background {
					wg.Add(1)
				} else if g.collectOrphans {
					g.orphans.Add(1)
				}

				go func(ctx context.Context, g *Graph, key string, f func(context.Context) error) {
					now := time.Now()
					err := f(ctx)
					g.ops[key].Lock()
					if err != nil {
						g.ops[key].err = multierror.Append(g.ops[key].err, err)
					}
					g.ops[key].executed = true

					if !g.ops[key].background {
						wg.Done()
					} else if g.collectOrphans {
						g.orphans.Done()
					}
					g.ops[key].duration = time.Since(now)
					g.ops[key].Unlock()
				}(ctx, g, r.Name, fns[i])
			}
		}

		wg.Wait()
		if err := checkFatal(layer); err != nil {
			return err
		}
	}

	if g.collectOrphans {
		g.orphans.Wait()
		for _, layer := range g.buildStateGraph() {
			if err := checkFatal(layer); err != nil {
				return err
			}
		}
	}
	return nil
}

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
	duration   time.Duration
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
		Duration:         o.duration,
	}
}

// GraphOption it's the option for the DAG graph.
type GraphOption func(g *Graph)

// EnableInit enables an Init jobs that takes paternity
// of orphan jobs without dependencies.
var EnableInit GraphOption = func(g *Graph) {
	g.init = true
}

// CollectOrphans enables orphan job collection.
var CollectOrphans GraphOption = func(g *Graph) {
	g.collectOrphans = true
}

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

// WithCallback associates a callback to the operation to be executed
// when the DAG is walked-by.
func WithCallback(fn ...func(context.Context) error) OpOption {
	return func(s string, os *OpState, g *Graph) error {
		os.fn = append(os.fn, fn...)
		return nil
	}
}
