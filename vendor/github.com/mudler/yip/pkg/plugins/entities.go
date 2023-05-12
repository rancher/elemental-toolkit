package plugins

import (
	"github.com/hashicorp/go-multierror"
	entities "github.com/mudler/entities/pkg/entities"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func Entities(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	if len(s.EnsureEntities) > 0 {
		if err := ensureEntities(l, s); err != nil {
			l.Error(err.Error())
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func DeleteEntities(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	if len(s.DeleteEntities) > 0 {
		if err := deleteEntities(l, s); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func deleteEntities(l logger.Interface, s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.DeleteEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(l, e.Entity)))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Delete(e.Path)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func ensureEntities(l logger.Interface, s schema.Stage) error {
	var errs error
	entityParser := entities.Parser{}
	for _, e := range s.EnsureEntities {
		decodedE, err := entityParser.ReadEntityFromBytes([]byte(templateSysData(l, e.Entity)))
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = decodedE.Apply(e.Path, false)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}
