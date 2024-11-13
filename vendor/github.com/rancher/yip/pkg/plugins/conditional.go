package plugins

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
)

func NodeConditional(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Node) > 0 {
		matched, err := regexp.MatchString(s.Node, system.Node.Hostname)
		if !matched {
			return fmt.Errorf("Skipping stage (node hostname '%s' doesn't match '%s')", system.Node.Hostname, s.Node)
		}
		if err != nil {
			return errors.Join(err, fmt.Errorf("Skipping invalid regex for node hostname '%s'", s.Node))
		}
	}
	return nil
}
