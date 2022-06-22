package plugins

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func Commands(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, cmd := range s.Commands {
		out, err := console.Run(templateSysData(l, cmd))
		if err != nil {
			l.Error(out, ": ", err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
		l.Info(fmt.Sprintf("Command output: %s", string(out)))
	}
	return errs
}
