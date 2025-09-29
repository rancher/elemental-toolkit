//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/dag"
	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/plugins"
	"github.com/rancher/yip/pkg/schema"
	"github.com/rancher/yip/pkg/utils"
)

// DefaultExecutor is the default yip Executor.
// It simply creates file and executes command for a linux executor
type DefaultExecutor struct {
	plugins      []Plugin
	conditionals []Plugin
	modifier     schema.Modifier
	logger       logger.Interface
}

func (e *DefaultExecutor) Plugins(p []Plugin) {
	e.plugins = p
}

func (e *DefaultExecutor) Conditionals(p []Plugin) {
	e.conditionals = p
}

func (e *DefaultExecutor) Modifier(m schema.Modifier) {
	e.modifier = m
}

type op struct {
	fn      func(context.Context) error
	deps    []string
	after   []string
	options []dag.OpOption
	name    string
}

type opList []*op

func (l opList) uniqueNames() {
	names := map[string]int{}

	for _, op := range l {
		if names[op.name] > 0 {
			op.name = fmt.Sprintf("%s.%d", op.name, names[op.name])
			names[op.name] = names[op.name] + 1
		} else {
			names[op.name] = 1
		}
	}
}

type loaderError struct {
	path string
}

func (e loaderError) Error() string {
	return fmt.Sprintf("error loading path '%s'", e.path)
}

func newLoaderError(path string) loaderError {
	return loaderError{
		path,
	}
}

func (e *DefaultExecutor) applyStage(stage schema.Stage, fs vfs.FS, console plugins.Console) error {
	var errs error
	for _, p := range e.conditionals {
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Warnf("(conditional) Skip '%s' stage name: %s",
				err.Error(), stage.Name)
			return nil
		}
	}

	e.logger.Infof(
		"Processing stage step '%s'. ( commands: %d, files: %d, ... )",
		stage.Name,
		len(stage.Commands),
		len(stage.Files))

	b, _ := json.Marshal(stage)
	e.logger.Debugf("Stage: %s", string(b))

	fmt.Printf("Processing stage step %s \n", stage.Name)

	for _, p := range e.plugins {
		if err := p(e.logger, stage, fs, console); err != nil {
			e.logger.Error(err.Error())
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func checkDuplicates(stages []schema.Stage) bool {
	stageNames := map[string]bool{}
	for _, st := range stages {
		if _, ok := stageNames[st.Name]; ok {
			return true
		}
		stageNames[st.Name] = true
	}
	return false
}

func (e *DefaultExecutor) genOpFromSchema(file, stage string, config schema.YipConfig, fs vfs.FS, console plugins.Console) []*op {
	results := []*op{}
	currentStages := config.Stages[stage]

	duplicatedNames := checkDuplicates(currentStages)

	prev := ""
	for i, st := range currentStages {
		name := st.Name
		if duplicatedNames {
			name = fmt.Sprintf("%s.%d", st.Name, i)
		}

		if name == "" {
			name = fmt.Sprint(i)
		}

		rootname := file
		if config.Name != "" {
			rootname = config.Name
		}

		// Copy here so it doesn't get overwritten and points to the same state
		stageLocal := st
		opName := fmt.Sprintf("%s.%s", rootname, name)

		e.logger.Debugf("Generating op for stage '%s'", opName)
		o := &op{
			fn: func(ctx context.Context) error {
				e.logger.Debugf("Reading '%s'", file)
				e.logger.Debugf("Executing stage '%s'", opName)
				return e.applyStage(stageLocal, fs, console)
			},
			name:    opName,
			options: []dag.OpOption{dag.WeakDeps},
		}

		for _, d := range st.After {
			o.after = append(o.after, d.Name)
		}

		if i != 0 && len(st.After) == 0 {
			o.deps = append(o.deps, prev)
		}

		results = append(results, o)

		prev = opName
	}

	return results
}

func (e *DefaultExecutor) dirOps(stage, dir string, fs vfs.FS, console plugins.Console) (results []*op, loaderError error, err error) {
	results = []*op{}
	prev := []*op{}
	err = vfs.Walk(fs, dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == dir {
				return nil
			}
			// Process only files
			if info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}

			config, err := schema.Load(path, fs, schema.FromFile, e.modifier)
			if err != nil {
				e.logger.Warnf("failed to load file '%s': %s", path, err.Error())
				loaderError = multierror.Append(loaderError, newLoaderError(path))
				return nil
			}

			ops := e.genOpFromSchema(path, stage, *config, fs, console)
			// mark lexicographic order dependency from previous blocks
			if len(prev) > 0 && len(ops) > 0 {
				for _, p := range prev {
					if len(p.after) == 0 {
						for _, o := range ops {
							o.deps = append(o.deps, p.name)
						}
					}
				}
			}
			prev = ops

			// append results
			results = append(results, ops...)
			return nil
		})

	return results, loaderError, err
}

func writeDAG(dag [][]dag.GraphEntry) {
	for i, layer := range dag {
		fmt.Printf("%d.\n", (i + 1))
		for _, op := range layer {
			if op.Error != nil {
				fmt.Printf(" <%s> (error: %s) (background: %t) (weak: %t)\n", op.Name, op.Error.Error(), op.Background, op.WeakDeps)
			} else {
				fmt.Printf(" <%s> (background: %t) (weak: %t)\n", op.Name, op.Background, op.WeakDeps)
			}
		}
	}
	return
}

func (e *DefaultExecutor) Graph(stage string, fs vfs.FS, console plugins.Console, source string) ([][]dag.GraphEntry, error) {
	g, loaderError, err := e.prepareDAG(stage, source, fs, console)
	if err != nil {
		return nil, err
	}
	return g.Analyze(), loaderError
}

func (e *DefaultExecutor) Analyze(stage string, fs vfs.FS, console plugins.Console, args ...string) {
	var errs error
	for _, source := range args {
		g, _, err := e.prepareDAG(stage, source, fs, console)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		for i, layer := range g.Analyze() {
			e.logger.Infof("%d.", (i + 1))
			for _, op := range layer {
				if op.Error != nil {
					e.logger.Infof(" <%s> (error: %s) (background: %t) (weak: %t)", op.Name, op.Error.Error(), op.Background, op.WeakDeps)
				} else {
					e.logger.Infof(" <%s> (background: %t) (weak: %t)", op.Name, op.Background, op.WeakDeps)
				}
			}
		}
	}
}

func dumpOps(uri string, ops opList) {
	fmt.Printf("the uri is %v\n", uri)
	for _, op := range ops {
		fmt.Printf("%+v\n", op)
	}
}

func (e *DefaultExecutor) prepareDAG(stage, uri string, fs vfs.FS, console plugins.Console) (g *dag.Graph, loaderError error, err error) {
	f, err := fs.Stat(uri)

	g = dag.DAG(dag.EnableInit)
	var ops opList
	switch {
	case err == nil && f.IsDir():
		ops, loaderError, err = e.dirOps(stage, uri, fs, console)
		if err != nil {
			return nil, loaderError, err
		}
		dumpOps(uri, ops)
	case err == nil:
		config, err := schema.Load(uri, fs, schema.FromFile, e.modifier)
		if err != nil {
			return nil, nil, err
		}
		ops = e.genOpFromSchema(uri, stage, *config, fs, console)
		dumpOps(uri, ops)
	case utils.IsUrl(uri):
		config, err := schema.Load(uri, fs, schema.FromUrl, e.modifier)
		if err != nil {
			return nil, nil, err
		}

		ops = e.genOpFromSchema(uri, stage, *config, fs, console)
		dumpOps(uri, ops)
	default:
		config, err := schema.Load(uri, fs, nil, e.modifier)
		if err != nil {
			return nil, nil, err
		}

		ops = e.genOpFromSchema("<STDIN>", stage, *config, fs, console)
		dumpOps(uri, ops)
	}

	// Ensure all names are unique
	ops.uniqueNames()
	for _, o := range ops {
		g.Add(o.name, append(o.options, dag.WithCallback(o.fn), dag.WithDeps(append(o.after, o.deps...)...))...)
	}

	return g, loaderError, nil
}

func (e *DefaultExecutor) runStage(stage, uri string, fs vfs.FS, console plugins.Console) (err error) {
	g, loaderError, err := e.prepareDAG(stage, uri, fs, console)
	if err != nil {
		return err
	}

	if g == nil {
		return fmt.Errorf("no dag could be created")
	}

	err = g.Run(context.Background())
	if err != nil {
		return err
	}

	for _, g := range g.Analyze() {
		for _, gg := range g {
			if gg.Error != nil {
				err = multierror.Append(err, gg.Error)
			}
		}
	}

	if loaderError != nil {
		err = multierror.Append(err, loaderError)
	}

	return err
}

// Run takes a list of URI to run yipfiles from. URI can be also a dir or a local path, as well as a remote
func (e *DefaultExecutor) Run(stage string, fs vfs.FS, console plugins.Console, args ...string) error {
	var errs error
	e.logger.Infof("Running stage: %s\n", stage)
	for _, source := range args {
		if err := e.runStage(stage, source, fs, console); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	e.logger.Infof("Done executing stage '%s'\n", stage)
	return errs
}

// Apply applies a yip Config file by creating files and running commands defined.
func (e *DefaultExecutor) Apply(stageName string, s schema.YipConfig, fs vfs.FS, console plugins.Console) error {
	currentStages := s.Stages[stageName]
	if len(currentStages) == 0 {
		e.logger.Debugf("No commands to run for %s %s\n", stageName, s.Name)
		return nil
	}

	e.logger.Infof("Applying '%s' for stage '%s'. Total stages: %d\n", s.Name, stageName, len(currentStages))

	var errs error
STAGES:
	for _, stage := range currentStages {
		for _, p := range e.conditionals {
			if err := p(e.logger, stage, fs, console); err != nil {
				e.logger.Warnf("Error '%s' in stage name: %s stage: %s\n",
					err.Error(), s.Name, stageName)
				continue STAGES
			}
		}

		e.logger.Infof(
			"Processing stage step '%s'. ( commands: %d, files: %d, ... )\n",
			stage.Name,
			len(stage.Commands),
			len(stage.Files))

		b, _ := json.Marshal(stage)
		e.logger.Debugf("Stage: %s", string(b))

		fmt.Printf("Processing stage step %s \n", stage.Name)

		for _, p := range e.plugins {
			if err := p(e.logger, stage, fs, console); err != nil {
				e.logger.Error(err.Error())
				errs = multierror.Append(errs, err)
			}
		}
	}

	e.logger.Infof(
		"Stage '%s'. Defined stages: %d. Errors: %t\n",
		stageName,
		len(currentStages),
		errs != nil,
	)

	return errs
}
