# :checkered_flag:  herd

[![Go Reference](https://pkg.go.dev/badge/github.com/spectrocloud-labs/herd.svg)](https://pkg.go.dev/github.com/spectrocloud-labs/herd)
[![Lint](https://github.com/spectrocloud-labs/herd/actions/workflows/lint.yaml/badge.svg)](https://github.com/spectrocloud-labs/herd/actions/workflows/lint.yaml)
[![Unit tests](https://github.com/spectrocloud-labs/herd/actions/workflows/test.yaml/badge.svg)](https://github.com/spectrocloud-labs/herd/actions/workflows/test.yaml)

Herd is a Embedded Runnable DAG (H.E.R.D.). it aims to be a tiny library that allows to define arbitrary DAG, and associate job operations on them.

## Why?

I've found couple of nice libraries ([fx](https://github.com/uber-go/fx), or [dag](https://github.com/mostafa-asg/dag) for instance), however none of them satisfied my constraints:

- Tiny
- Completely tested (TDD)
- Define jobs in a DAG, runs them in sequence, execute the ones that can be done in parallel (parallel topological sorting) in separate go routines
- Provide some sorta of similarity with `systemd` concepts

## Usage

`herd` can be used as a library as such:

```golang
package main

import (
    "context"

    "github.com/spectrocloud-labs/herd"
)

func main() {

    // Generic usage
    g := herd.DAG()
    g.Add("name", ...)
    g.Run(context.TODO())

    // Example
    f := ""
    g.Add("foo", herd.WithCallback(func(ctx context.Context) error {
        f += "foo"
        // This executes after "bar" has ended successfully.
        return nil
    }), herd.WithDeps("bar"))

    g.Add("bar", herd.WithCallback(func(ctx context.Context) error {
        f += "bar"
        // This execute first
        return nil
    }))

    // Execute the DAG
    g.Run(context.Background())
    // f is "barfoo"
}

```