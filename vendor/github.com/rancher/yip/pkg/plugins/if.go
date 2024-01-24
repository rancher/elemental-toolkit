package plugins

import (
	"fmt"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
)

func IfConditional(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.If) > 0 {
		out, err := console.Run(templateSysData(l, s.If))
		if err != nil {
			return fmt.Errorf("Skipping stage (if statement error: %w)", err)
		}
		l.Debugf("If statement result %s", out)
	}
	return nil
}
