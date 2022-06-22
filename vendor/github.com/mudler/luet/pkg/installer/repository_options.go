// Copyright © 2021 Ettore Di Giacinto <mudler@sabayon.org>
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

package installer

import (
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/tree"
)

type RepositoryOption func(cfg *RepositoryConfig) error

type RepositoryConfig struct {
	Name, Description, Type string
	Urls                    []string
	Priority                int
	Src                     string
	Tree                    []string
	DB                      types.PackageDatabase
	CompilerBackend         compiler.CompilerBackend
	ImagePrefix             string

	context                                         types.Context
	PushImages, Force, FromRepository, FromMetadata bool

	compilerParser []tree.FileParser
	runtimeParser  []tree.FileParser
}

// Apply applies the given options to the config, returning the first error
// encountered (if any).
func (cfg *RepositoryConfig) Apply(opts ...RepositoryOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

func WithContext(c types.Context) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.context = c
		return nil
	}
}

func WithRuntimeParser(parsers ...tree.FileParser) RepositoryOption {
	return func(cfg *RepositoryConfig) error {
		cfg.runtimeParser = append(cfg.runtimeParser, parsers...)
		return nil
	}
}

func WithCompilerParser(parsers ...tree.FileParser) RepositoryOption {
	return func(cfg *RepositoryConfig) error {
		cfg.compilerParser = append(cfg.compilerParser, parsers...)
		return nil
	}
}

func WithDatabase(b types.PackageDatabase) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.DB = b
		return nil
	}
}

func WithCompilerBackend(b compiler.CompilerBackend) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.CompilerBackend = b
		return nil
	}
}

func WithTree(s ...string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Tree = append(cfg.Tree, s...)
		return nil
	}
}

func WithUrls(s ...string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Urls = append(cfg.Urls, s...)
		return nil
	}
}

func WithSource(s string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Src = s
		return nil
	}
}

func WithName(s string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Name = s
		return nil
	}
}

func WithDescription(s string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Description = s
		return nil
	}
}

func WithType(s string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Type = s
		return nil
	}
}

func WithImagePrefix(s string) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.ImagePrefix = s
		return nil
	}
}

func WithPushImages(b bool) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.PushImages = b
		return nil
	}
}

func WithForce(b bool) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Force = b
		return nil
	}
}

// FromRepository when enabled
// considers packages metadata
// from remote repositories when building
// the new repository index
func FromRepository(b bool) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.FromRepository = b
		return nil
	}
}

// FromMetadata when enabled
// considers packages metadata
// when building repository indexes
func FromMetadata(b bool) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.FromMetadata = b
		return nil
	}
}

func WithPriority(b int) func(cfg *RepositoryConfig) error {
	return func(cfg *RepositoryConfig) error {
		cfg.Priority = b
		return nil
	}
}
