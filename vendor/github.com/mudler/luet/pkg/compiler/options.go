// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package compiler

import (
	"runtime"

	"github.com/mudler/luet/pkg/api/core/types"
)

func newDefaultCompiler() *types.CompilerOptions {
	return &types.CompilerOptions{
		PushImageRepository: "luet/cache",
		PullFirst:           false,
		Push:                false,
		CompressionType:     types.None,
		KeepImg:             true,
		Concurrency:         runtime.NumCPU(),
		OnlyDeps:            false,
		NoDeps:              false,
		SolverOptions:       types.LuetSolverOptions{SolverOptions: types.SolverOptions{Concurrency: 1, Type: types.SolverSingleCoreSimple}},
	}
}

func WithOptions(opt *types.CompilerOptions) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg = opt
		return nil
	}
}

func WithRuntimeDatabase(db types.PackageDatabase) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.RuntimeDatabase = db
		return nil
	}
}

// WithFinalRepository Sets the final repository where to push
// images of built artifacts
func WithFinalRepository(r string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.PushFinalImagesRepository = r
		return nil
	}
}

func EnableGenerateFinalImages(cfg *types.CompilerOptions) error {
	cfg.GenerateFinalImages = true
	return nil
}

func EnablePushFinalImages(cfg *types.CompilerOptions) error {
	cfg.PushFinalImages = true
	return nil
}

func ForcePushFinalImages(cfg *types.CompilerOptions) error {
	cfg.PushFinalImagesForce = true
	return nil
}

func WithBackendType(r string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.BackendType = r
		return nil
	}
}

func WithTemplateFolder(r []string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.TemplatesFolder = r
		return nil
	}
}

func WithBuildValues(r []string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.BuildValuesFile = r
		return nil
	}
}

func WithPullRepositories(r []string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.PullImageRepository = r
		return nil
	}
}

// WithPushRepository Sets the image reference where to push
// cache images
func WithPushRepository(r string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		if len(cfg.PullImageRepository) == 0 {
			cfg.PullImageRepository = []string{cfg.PushImageRepository}
		}
		cfg.PushImageRepository = r
		return nil
	}
}

func BackendArgs(r []string) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.BackendArgs = r
		return nil
	}
}

func PullFirst(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.PullFirst = b
		return nil
	}
}

func KeepImg(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.KeepImg = b
		return nil
	}
}

func Rebuild(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.Rebuild = b
		return nil
	}
}

func PushImages(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.Push = b
		return nil
	}
}

func Wait(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.Wait = b
		return nil
	}
}

func OnlyDeps(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.OnlyDeps = b
		return nil
	}
}

func OnlyTarget(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.PackageTargetOnly = b
		return nil
	}
}

func NoDeps(b bool) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.NoDeps = b
		return nil
	}
}

func Concurrency(i int) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		if i == 0 {
			i = runtime.NumCPU()
		}
		cfg.Concurrency = i
		return nil
	}
}

func WithCompressionType(t types.CompressionImplementation) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.CompressionType = t
		return nil
	}
}

func WithSolverOptions(c types.LuetSolverOptions) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.SolverOptions = c
		return nil
	}
}

func WithContext(c types.Context) func(cfg *types.CompilerOptions) error {
	return func(cfg *types.CompilerOptions) error {
		cfg.Context = c
		return nil
	}
}
