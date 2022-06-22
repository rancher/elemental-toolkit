package compiler

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/compiler/backend"
	"github.com/pkg/errors"
)

func NewBackend(ctx types.Context, s string) (CompilerBackend, error) {
	var compilerBackend CompilerBackend

	switch s {
	case backend.ImgBackend:
		compilerBackend = backend.NewSimpleImgBackend(ctx)
	case backend.DockerBackend:
		compilerBackend = backend.NewSimpleDockerBackend(ctx)
	default:
		return nil, errors.New("invalid backend. Unsupported")
	}

	return compilerBackend, nil
}

type CompilerBackend interface {
	BuildImage(backend.Options) error
	ExportImage(backend.Options) error
	LoadImage(string) error
	RemoveImage(backend.Options) error
	ImageDefinitionToTar(backend.Options) error

	CopyImage(string, string) error
	DownloadImage(opts backend.Options) error

	Push(opts backend.Options) error
	ImageAvailable(string) bool

	ImageReference(img1 string, ondisk bool) (v1.Image, error)
	ImageExists(string) bool
}
