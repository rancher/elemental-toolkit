topsort
=======

Topological Sorting for Golang

Topological sorting algorithms are especially useful for dependency calculation, and so this particular implementation is mainly intended for this purpose. As a result, the direction of edges and the order of the results may seem reversed compared to other implementations of topological sorting.

For example, if:

* A depends on B
* B depends on C

The graph is represented as:

```
A -> B -> C
```

Where `->` represents a directed edge from one node to another.

The topological ordering of dependencies results in:

```
[C, B, A]
```

The code for this example would look something like:

```go
// Initialize the graph.
graph := topsort.NewGraph()
graph.AddNode("A")
graph.AddNode("B")
graph.AddNode("C")

// Add edges.
graph.AddEdge("A", "B")
graph.AddEdge("B", "C")

// Topologically sort node A.
graph.TopSort("A")  // => [C, B, A]
```
