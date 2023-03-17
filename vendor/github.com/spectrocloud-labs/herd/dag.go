package herd

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/kendru/darwin/go/depgraph"
	"github.com/samber/lo"
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
					if len(r.WeakDependencies) != 0 && lo.Contains(r.WeakDependencies, k) {
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
