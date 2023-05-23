package plugins

import (
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func Systemctl(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	if err := console.RunTemplate(s.Systemctl.Enable, "systemctl enable %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Disable, "systemctl disable %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Mask, "systemctl mask %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := console.RunTemplate(s.Systemctl.Start, "systemctl start %s"); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}
