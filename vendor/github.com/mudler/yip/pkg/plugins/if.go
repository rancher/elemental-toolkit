package plugins

import (
	"fmt"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func IfConditional(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.If) > 0 {
		out, err := console.Run(templateSysData(l, s.If))
		if err != nil {
			return fmt.Errorf("Skipping stage (if statement didn't passed)")
		}
		l.Debugf("If statement result %s", out)
	}
	return nil
}
