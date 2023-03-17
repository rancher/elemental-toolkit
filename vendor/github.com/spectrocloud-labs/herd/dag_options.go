package herd

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
