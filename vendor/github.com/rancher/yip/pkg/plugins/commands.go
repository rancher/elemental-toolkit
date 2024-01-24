package plugins

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
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
