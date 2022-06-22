// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package context

import (
	"context"
	"os"
	"path/filepath"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	gc "github.com/mudler/luet/pkg/api/core/garbagecollector"
	"github.com/mudler/luet/pkg/api/core/logger"
	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/pkg/errors"
)

type Context struct {
	types.Logger
	context.Context
	types.GarbageCollector
	Config      *types.LuetConfig
	NoSpinner   bool
	annotations map[string]interface{}
}

// SetAnnotation sets generic annotations to hold in a context
func (c *Context) SetAnnotation(s string, i interface{}) {
	c.annotations[s] = i
}

// GetAnnotation gets generic annotations to hold in a context
func (c *Context) GetAnnotation(s string) interface{} {
	return c.annotations[s]
}

type ContextOption func(c *Context) error

// WithLogger sets the logger
func WithLogger(l types.Logger) ContextOption {
	return func(c *Context) error {
		c.Logger = l
		return nil
	}
}

// WithConfig sets the luet config
func WithConfig(cc *types.LuetConfig) ContextOption {
	return func(c *Context) error {
		c.Config = cc
		return nil
	}
}

// NOTE: GC needs to be instantiated when a new context is created from system TmpDirBase

// WithGarbageCollector sets the Garbage collector for the given context
func WithGarbageCollector(l types.GarbageCollector) ContextOption {
	return func(c *Context) error {
		if !filepath.IsAbs(l.String()) {
			abs, err := fileHelper.Rel2Abs(l.String())
			if err != nil {
				return errors.Wrap(err, "while converting relative path to absolute path")
			}
			l = gc.GarbageCollector(abs)
		}

		c.GarbageCollector = l
		return nil
	}
}

// NewContext returns a new context.
// It accepts a Garbage collector, a config and a logger as an option
func NewContext(opts ...ContextOption) *Context {
	l, _ := logger.New()
	d := &Context{
		annotations:      make(map[string]interface{}),
		Logger:           l,
		GarbageCollector: gc.GarbageCollector(filepath.Join(os.TempDir(), "tmpluet")),
		Config: &types.LuetConfig{
			ConfigFromHost: true,
			Logging:        types.LuetLoggingConfig{},
			General:        types.LuetGeneralConfig{},
			System: types.LuetSystemConfig{
				DatabasePath:  filepath.Join("var", "db"),
				PkgsCachePath: filepath.Join("var", "db", "packages"),
			},
			Solver: types.LuetSolverOptions{},
		},
	}

	for _, o := range opts {
		o(d)
	}
	return d
}

// WithLoggingContext returns a copy of the context with a contextualized logger
func (c *Context) WithLoggingContext(name string) types.Context {
	configCopy := *c.Config
	configCopy.System = c.Config.System
	configCopy.General = c.Config.General
	configCopy.Logging = c.Config.Logging

	ctx := *c
	ctxCopy := &ctx
	ctxCopy.Config = &configCopy
	ctxCopy.annotations = ctx.annotations

	ctxCopy.Logger, _ = c.Logger.Copy()
	ctxCopy.Logger.SetContext(name)

	return ctxCopy
}

// Copy returns a context copy with a reset logging context
func (c *Context) Clone() types.Context {
	return c.WithLoggingContext("")
}

func (c *Context) Warning(mess ...interface{}) {
	c.Logger.Warn(mess...)
	if c.Config.General.FatalWarns {
		panic("panic on warning")
	}
}

func (c *Context) Warn(mess ...interface{}) {
	c.Warning(mess...)
}

func (c *Context) Warnf(t string, mess ...interface{}) {
	c.Logger.Warnf(t, mess...)
	if c.Config.General.FatalWarns {
		panic("panic on warning")
	}
}

func (c *Context) Warningf(t string, mess ...interface{}) {
	c.Warnf(t, mess...)
}

func (c *Context) GetConfig() types.LuetConfig {
	return *c.Config
}
