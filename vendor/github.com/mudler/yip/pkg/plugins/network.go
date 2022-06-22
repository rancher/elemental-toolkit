package plugins

import (
	"fmt"
	"regexp"

	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
)

func NodeConditional(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Node) > 0 {
		matched, err := regexp.MatchString(s.Node, system.Node.Hostname)
		if !matched {
			return fmt.Errorf("Skipping stage (node hostname '%s' doesn't match '%s')", system.Node.Hostname, s.Node)
		}
		if err != nil {
			return errors.Wrapf(err, "Skipping invalid regex for node hostname '%s', error: %s", s.Node, err.Error())
		}
	}
	return nil
}
